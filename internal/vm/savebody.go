// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/githugt/flowi-lib/engine"
)

// SavedBody is a response body ready to be written to a file: its bytes and a
// name to offer in the save dialog.
type SavedBody struct {
	Data []byte
	Name string
}

// ResponseBodyFile is the body of a recorded response as bytes, and a file name
// for it: the last part of the URL's path when it has an extension, else
// "response" with the extension of the content type (when it has one). A body
// that was streamed to a file is read from there.
func ResponseBodyFile(rec engine.Record) (SavedBody, error) {
	data, err := bodyBytes(rec.Body)
	if err != nil {
		return SavedBody{}, err
	}
	return SavedBody{Data: data, Name: fileNameFor(rec.Request.URL, rec.Body.ContentType)}, nil
}

func bodyBytes(b engine.Body) ([]byte, error) {
	switch {
	case b.File != "":
		data, err := os.ReadFile(b.File)
		if err != nil {
			return nil, fmt.Errorf("reading the saved download: %w", err)
		}
		return data, nil
	case b.Base64 != "":
		data, err := base64.StdEncoding.DecodeString(b.Base64)
		if err != nil {
			return nil, fmt.Errorf("decoding the body: %w", err)
		}
		return data, nil
	case b.Text != "":
		return []byte(b.Text), nil
	case b.JSON != nil:
		return json.MarshalIndent(b.JSON, "", "  ")
	}
	return nil, errors.New("the response has no body to save")
}

// fileNameFor names a saved body. The URL's last path segment is used only
// when it is a plain name with an extension; otherwise the name comes from the
// content type, so a hostile URL cannot choose where or under what name the file
// is offered.
func fileNameFor(rawURL, contentType string) string {
	last := ""
	if u, err := url.Parse(rawURL); err == nil {
		last = path.Base(u.Path)
	}
	if plain(last) {
		if path.Ext(last) != "" {
			return last
		}
		return last + extensionOf(contentType)
	}
	return "response" + extensionOf(contentType)
}

// plain reports whether s is a harmless file name: not empty, not a directory
// marker and nothing that names another place.
func plain(s string) bool {
	return s != "" && s != "." && s != ".." && s != "/" && !strings.ContainsAny(s, `/\:*?"<>|`+"\x00")
}

// extensionOf is the extension (with its dot) a content type is usually saved
// with, or "" when it is not known.
func extensionOf(contentType string) string {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	for kind, ext := range map[string]string{
		"application/json": ".json", "application/xml": ".xml", "text/xml": ".xml", "text/html": ".html", "text/plain": ".txt",
		"text/csv": ".csv", "application/yaml": ".yaml", "application/pdf": ".pdf", "image/png": ".png", "image/jpeg": ".jpg",
		"image/gif": ".gif", "image/svg+xml": ".svg", "application/zip": ".zip",
	} {
		if media == kind {
			return ext
		}
	}
	if strings.HasSuffix(media, "+json") {
		return ".json"
	}
	return ""
}
