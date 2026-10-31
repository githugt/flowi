// SPDX-License-Identifier: Apache-2.0

// Package problem checks that an error response is a problem details document (RFC 9457): the media type
// application/problem+json, a JSON object, and the standard members (type, title, status, detail, instance) used as the
// RFC says. Extension members are allowed and not looked at.
package problem

import (
	"fmt"
	"mime"
	"net/url"
	"strings"
)

// Violation is one way a response is not a problem details document. Path is "Content-Type" or a JSON path.
type Violation struct {
	Path    string
	Message string
}

// IsProblemType says whether a Content-Type is application/problem+json or application/problem+xml.
func IsProblemType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == "application/problem+json" || mediaType == "application/problem+xml")
}

// Check lists what is wrong with a response of the given HTTP status, Content-Type and decoded JSON body; none means
// it is a problem details document. An XML problem document is accepted by its media type and its body is not read.
func Check(status int, contentType string, body any) []Violation {
	var out []Violation
	if !IsProblemType(contentType) {
		out = append(out, Violation{Path: "Content-Type", Message: fmt.Sprintf("is %q, want application/problem+json", contentType)})
	}
	if mediaType, _, _ := mime.ParseMediaType(contentType); mediaType == "application/problem+xml" {
		return out
	}
	object, ok := body.(map[string]any)
	if !ok {
		return append(out, Violation{Path: "$", Message: "the body is not a JSON object"})
	}
	return append(out, membersOf(status, object)...)
}

func membersOf(status int, object map[string]any) []Violation {
	var out []Violation
	add := func(name, message string) { out = append(out, Violation{Path: "$." + name, Message: message}) }
	for _, name := range []string{"type", "instance"} {
		if v, present := object[name]; present {
			if s, isString := v.(string); !isString {
				add(name, "is not a string (a URI reference)")
			} else if !isURIReference(s) {
				add(name, fmt.Sprintf("%q is not a URI reference", s))
			}
		}
	}
	for _, name := range []string{"title", "detail"} {
		if v, present := object[name]; present {
			if _, isString := v.(string); !isString {
				add(name, "is not a string")
			}
		}
	}
	if v, present := object["status"]; present {
		code, isNumber := numberOf(v)
		switch {
		case !isNumber:
			add("status", "is not a number")
		case code != status:
			add("status", fmt.Sprintf("is %d but the HTTP status is %d", code, status))
		}
	}
	return out
}

// isURIReference is true for text that parses as a URI reference and holds no space or control character.
func isURIReference(s string) bool {
	if strings.ContainsFunc(s, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return false
	}
	_, err := url.Parse(s)
	return err == nil
}

// numberOf reads a JSON number that is a whole number, whichever Go type the decoder gave it.
func numberOf(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), float64(int(n)) == n
	case interface{ Int64() (int64, error) }: // json.Number
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}
