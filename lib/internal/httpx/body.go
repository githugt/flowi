// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// BodySource opens a request body. It is opened once per attempt, so
// retries resend the whole body. length is -1 when unknown.
type BodySource interface {
	Open() (r io.ReadCloser, length int64, contentType string, err error)
}

type bytesBody struct {
	data        []byte
	contentType string
}

// BytesBody is a body held in memory.
func BytesBody(b []byte, contentType string) BodySource {
	return bytesBody{data: b, contentType: contentType}
}

func (b bytesBody) Open() (io.ReadCloser, int64, string, error) {
	return io.NopCloser(bytes.NewReader(b.data)), int64(len(b.data)), b.contentType, nil
}

// openedBody is a body ready to attach to a request.
type openedBody struct {
	reader      io.ReadCloser // nil for no body
	length      int64
	contentType string
	encoding    string
}

func openBody(src BodySource, compression string) (openedBody, error) {
	if src == nil {
		return openedBody{}, nil
	}
	r, n, ctype, err := src.Open()
	if err != nil {
		return openedBody{}, err
	}
	b := openedBody{reader: r, length: n, contentType: ctype}
	if compression == "gzip" {
		b = gzipped(b)
	}
	return b, nil
}

// gzipped compresses the body on the fly. The goroutine it starts owns the
// pipe's write side and exits when the body is fully read or the reader
// side is closed (the transport closes it when the request ends).
func gzipped(b openedBody) openedBody {
	pr, pw := io.Pipe()
	go func() {
		zw := gzip.NewWriter(pw)
		_, err := io.Copy(zw, b.reader)
		if err == nil {
			err = zw.Close()
		}
		closeQuietly(b.reader)
		pw.CloseWithError(err) //nolint:errcheck // CloseWithError always returns nil
	}()
	return openedBody{reader: pr, length: -1, contentType: b.contentType, encoding: "gzip"}
}

// apply sets the length and content headers the body implies, without
// overriding headers the caller set.
func (b openedBody) apply(req *http.Request) {
	if b.reader == nil {
		return
	}
	req.ContentLength = b.length
	if b.contentType != "" {
		req.Header.Set("Content-Type", mergeContentType(req.Header.Get("Content-Type"), b.contentType))
	}
	if b.encoding != "" {
		req.Header.Set("Content-Encoding", b.encoding)
	}
}

// mergeContentType picks the Content-Type to send. The caller's header
// wins, except a multipart type without a boundary: the server could not
// split the parts, so the boundary of the generated body is added.
func mergeContentType(user, generated string) string {
	if user == "" {
		return generated
	}
	mediaType, params, err := mime.ParseMediaType(user)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") || params["boundary"] != "" {
		return user
	}
	_, generatedParams, err := mime.ParseMediaType(generated)
	if err != nil || generatedParams["boundary"] == "" {
		return user
	}
	params["boundary"] = generatedParams["boundary"]
	return mime.FormatMediaType(mediaType, params)
}

// userBoundary returns the boundary a caller put in a multipart
// Content-Type, or "" when there is none.
func userBoundary(contentType string) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return ""
	}
	return params["boundary"]
}

// closeQuietly closes a body whose close error cannot change the outcome
// (it is being abandoned or was fully read).
func closeQuietly(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

// OutsideRootError means a file body path leaves the collection: an
// absolute path, "..", or a symlink pointing elsewhere. Collections are
// often cloned from others, so a request must not be able to upload
// arbitrary local files.
type OutsideRootError struct{ Path string }

func (e *OutsideRootError) Error() string { return "file " + e.Path + " is outside the collection" }

// resolveInRoot returns the resolved path of rel inside root, refusing paths
// that leave it.
func resolveInRoot(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", &OutsideRootError{Path: rel}
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolving collection root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(realRoot, rel))
	if err != nil {
		if inside(realRoot, filepath.Join(realRoot, rel)) {
			return "", err // a missing file inside the collection: report it as such
		}
		return "", &OutsideRootError{Path: rel}
	}
	if !inside(realRoot, resolved) {
		return "", &OutsideRootError{Path: rel}
	}
	return resolved, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type fileBody struct{ path string }

// FileBody sends a file of the collection. The path is checked now, before
// anything is sent; the file is reopened for every attempt.
func FileBody(root, rel string) (BodySource, error) {
	resolved, err := resolveInRoot(root, rel)
	if err != nil {
		return nil, err
	}
	return fileBody{path: resolved}, nil
}

func (f fileBody) Open() (io.ReadCloser, int64, string, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return nil, 0, "", err
	}
	info, err := file.Stat()
	if err != nil {
		closeQuietly(file)
		return nil, 0, "", err
	}
	return file, info.Size(), contentTypeOf(f.path), nil
}

func contentTypeOf(path string) string {
	if t := mime.TypeByExtension(filepath.Ext(path)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// FormField is one application/x-www-form-urlencoded field.
type FormField struct{ Name, Value string }

// FormBody encodes fields in the order given (url.Values would sort them).
func FormBody(fields []FormField) BodySource {
	pairs := make([]string, len(fields))
	for i, f := range fields {
		pairs[i] = url.QueryEscape(f.Name) + "=" + url.QueryEscape(f.Value)
	}
	return BytesBody([]byte(strings.Join(pairs, "&")), "application/x-www-form-urlencoded")
}

// Part is one multipart/form-data part: a value, or a file of the
// collection when File is set.
type Part struct {
	Name, Value       string
	File, ContentType string
}

type multipartBody struct {
	parts    []Part
	paths    []string // resolved file paths, parallel to parts ("" for values)
	boundary string   // "" picks a random one
}

// withBoundary returns the body written with the caller's boundary, so the
// body agrees with a Content-Type header the caller set.
func (m multipartBody) withBoundary(boundary string) BodySource {
	m.boundary = boundary
	return m
}

// MultipartBody streams parts as multipart/form-data. File paths are
// checked now, before anything is sent.
func MultipartBody(root string, parts []Part) (BodySource, error) {
	paths := make([]string, len(parts))
	for i, p := range parts {
		if p.File == "" {
			continue
		}
		resolved, err := resolveInRoot(root, p.File)
		if err != nil {
			return nil, err
		}
		paths[i] = resolved
	}
	return multipartBody{parts: parts, paths: paths}, nil
}

// Open starts a goroutine that writes the parts into a pipe. It owns the
// write side and exits when every part is written or the reader is
// closed (the transport closes it when the request ends).
func (m multipartBody) Open() (io.ReadCloser, int64, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	if m.boundary != "" {
		if err := mw.SetBoundary(m.boundary); err != nil {
			closeQuietly(pr)
			return nil, 0, "", fmt.Errorf("multipart boundary %q: %w", m.boundary, err)
		}
	}
	go func() {
		err := m.write(mw)
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err) //nolint:errcheck // CloseWithError always returns nil
	}()
	return pr, -1, mw.FormDataContentType(), nil
}

func (m multipartBody) write(mw *multipart.Writer) error {
	for i, p := range m.parts {
		if m.paths[i] == "" {
			if err := mw.WriteField(p.Name, p.Value); err != nil {
				return err
			}
			continue
		}
		if err := writeFilePart(mw, p, m.paths[i]); err != nil {
			return err
		}
	}
	return nil
}

func writeFilePart(mw *multipart.Writer, p Part, path string) error {
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": p.Name, "filename": filepath.Base(p.File)}))
	h.Set("Content-Type", cmpOrString(p.ContentType, contentTypeOf(path)))
	w, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer closeQuietly(f)
	_, err = io.Copy(w, f)
	return err
}

func cmpOrString(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
