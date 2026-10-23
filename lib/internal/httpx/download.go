// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/oklog/ulid/v2"
)

// receive reads and decodes the response body. Bodies up to
// StreamThreshold stay in memory; larger ones stream to a file in
// DownloadDir, which is created only when needed.
func (c *Client) receive(resp *http.Response) (*Response, error) {
	defer closeQuietly(resp.Body)
	raw := &countingReader{r: resp.Body}
	body, encoding, closeDecoder, err := decodedBody(resp.Header.Get("Content-Encoding"), raw)
	if err != nil {
		return nil, err
	}
	defer closeDecoder()
	got := &Response{Status: resp.StatusCode, Proto: resp.Proto, Header: resp.Header, Encoding: encoding}
	digest := sha256.New()
	head, err := io.ReadAll(io.LimitReader(io.TeeReader(body, digest), c.opts.StreamThreshold+1))
	if err != nil {
		return nil, err
	}
	if int64(len(head)) <= c.opts.StreamThreshold {
		got.Body, got.Size = head, int64(len(head))
	} else if err := c.stream(got, head, io.TeeReader(body, digest), resp.Header.Get("Content-Type")); err != nil {
		return nil, err
	}
	got.RawSize, got.SHA256 = raw.n, hex.EncodeToString(digest.Sum(nil))
	return got, nil
}

// decodedBody wraps raw in the decoder for encoding. An unknown encoding
// is kept as received and reported as "". An empty body is not decoded:
// servers send Content-Encoding on HEAD, 204 and 304 responses that have
// no body, and decoders would fail on it.
func decodedBody(encoding string, raw io.Reader) (io.Reader, string, func(), error) {
	buffered := bufio.NewReader(raw)
	if _, err := buffered.Peek(1); errors.Is(err, io.EOF) {
		return buffered, "", func() {}, nil
	}
	dec, known, err := decoder(encoding, buffered)
	if err != nil {
		return nil, "", nil, fmt.Errorf("decoding %s body: %w", encoding, err)
	}
	if !known {
		return buffered, "", func() {}, nil
	}
	return dec, encoding, func() { closeQuietly(dec) }, nil
}

// stream writes head and the rest of the body to a new file in
// DownloadDir, through a temp file renamed only when the whole body
// arrived, so a broken download leaves nothing behind.
func (c *Client) stream(got *Response, head []byte, rest io.Reader, contentType string) error {
	if err := os.MkdirAll(c.opts.DownloadDir, 0o750); err != nil {
		return fmt.Errorf("creating download directory: %w", err)
	}
	tmp, err := os.CreateTemp(c.opts.DownloadDir, ".download-*.tmp")
	if err != nil {
		return fmt.Errorf("creating download file: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // removes the temp file only if the rename below did not happen
	n, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(head), rest))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("downloading body: %w", err)
	}
	final := filepath.Join(c.opts.DownloadDir, ulid.Make().String()+extensionFor(contentType))
	if err := os.Rename(tmp.Name(), final); err != nil {
		return fmt.Errorf("saving download: %w", err)
	}
	got.File, got.Size = final, n
	return nil
}

func extensionFor(contentType string) string {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ".bin"
	}
	if exts, err := mime.ExtensionsByType(media); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}
