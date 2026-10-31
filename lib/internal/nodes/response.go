// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/theory/jsonpath"

	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/internal/xmlpath"
	"github.com/githugt/flowi-lib/model"
)

// errNothing means an extraction matched nothing.
var errNothing = errors.New("nothing matched")

// Extract reads one value from a response (or the cookie jar). It is
// shared by set values read from a response and a request version's
// extract: block.
func Extract(x model.Extraction, resp *Response, jar []httpx.StoredCookie) (any, error) {
	if resp == nil {
		return nil, errors.New("there is no response yet")
	}
	switch x.From {
	case "body":
		return fromBody(x, resp)
	case "header":
		return fromHeader(x, resp.Header)
	case "cookie":
		return fromCookie(x.Cookie, resp, jar)
	case "status":
		return resp.Status, nil
	}
	return nil, fmt.Errorf("unknown source %q (use body, header, cookie or status)", x.From)
}

func fromBody(x model.Extraction, resp *Response) (any, error) {
	if x.Regex != "" {
		return match(x.Regex, resp.Text)
	}
	if strings.HasPrefix(x.Path, "/") {
		return fromXML(x, resp) // a JSONPath starts with $, so a path that starts with / is for an XML body
	}
	p, err := jsonpath.Parse(x.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath %s: %w", x.Path, err)
	}
	found := p.Select(resp.Body)
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("%w at %s", errNothing, x.Path)
	case 1:
		return found[0], nil
	}
	return []any(found), nil
}

// fromXML reads a value from an XML body with a path (see package xmlpath): one match is the value, several are a list.
func fromXML(x model.Extraction, resp *Response) (any, error) {
	found, err := xmlpath.Select(resp.Text, x.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid XML path %s: %w", x.Path, err)
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("%w at %s", errNothing, x.Path)
	case 1:
		return found[0], nil
	}
	values := make([]any, len(found))
	for i, v := range found {
		values[i] = v
	}
	return values, nil
}

func fromHeader(x model.Extraction, h http.Header) (any, error) {
	v := h.Get(x.Header)
	if v == "" {
		return nil, fmt.Errorf("no header %s", x.Header)
	}
	if x.Regex != "" {
		return match(x.Regex, v)
	}
	return v, nil
}

func fromCookie(name string, resp *Response, jar []httpx.StoredCookie) (any, error) {
	for _, c := range resp.cookies() {
		if c.Name == name {
			return c.Value, nil
		}
	}
	for i := len(jar) - 1; i >= 0; i-- {
		if jar[i].Cookie.Name == name {
			return jar[i].Cookie.Value, nil
		}
	}
	return nil, fmt.Errorf("no cookie %s", name)
}

// match returns the first capture group, or the whole match without one.
func match(pattern, text string) (any, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	m := re.FindStringSubmatch(text)
	switch {
	case m == nil:
		return nil, fmt.Errorf("%w for /%s/", errNothing, pattern)
	case len(m) > 1:
		return m[1], nil
	}
	return m[0], nil
}
