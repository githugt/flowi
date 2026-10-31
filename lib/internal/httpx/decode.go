// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// decoder wraps r to decode a Content-Encoding. ok is false for an
// encoding it does not know (the body is then kept as received).
func decoder(encoding string, r io.Reader) (io.ReadCloser, bool, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(r)
		return zr, true, err
	case "deflate":
		zr, err := zlib.NewReader(r)
		return zr, true, err
	case "br":
		return io.NopCloser(brotli.NewReader(r)), true, nil
	case "zstd":
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, true, err
		}
		return zr.IOReadCloser(), true, nil
	}
	return nil, false, nil
}

// countingReader counts the bytes read through it (raw size on the wire).
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
