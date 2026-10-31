// SPDX-License-Identifier: Apache-2.0

// Package lifecycle reads what a server says about the end of an endpoint's life: the Deprecation header (RFC 9745),
// the Sunset header (RFC 8594) and the Link relations that point to the documentation and to the successor. A
// response that carries them is a warning to read before the day it stops working.
package lifecycle

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Notice is what a response announced.
type Notice struct {
	Deprecated bool
	// Since is when the endpoint was deprecated, zero when the server gave no date.
	Since time.Time
	// Sunset is when it will stop working, zero when the server gave none.
	Sunset       time.Time
	DocsURL      string // Link: rel="deprecation"
	SuccessorURL string // Link: rel="successor-version"
}

// Read reads the headers of a response. found is false when it announced nothing: a Sunset that is not a date counts as
// nothing, a Deprecation of any other value still means deprecated.
func Read(h http.Header) (n Notice, found bool) {
	if v := strings.TrimSpace(h.Get("Deprecation")); v != "" && !strings.EqualFold(v, "false") {
		n.Deprecated = true
		n.Since = deprecationDate(v)
	}
	if v := strings.TrimSpace(h.Get("Sunset")); v != "" {
		if t, err := http.ParseTime(v); err == nil {
			n.Sunset = t.UTC()
		}
	}
	found = n.Deprecated || !n.Sunset.IsZero()
	if !found {
		return Notice{}, false
	}
	for _, link := range links(h.Values("Link")) {
		switch link.rel {
		case "deprecation":
			n.DocsURL = link.url
		case "successor-version":
			n.SuccessorURL = link.url
		}
	}
	return n, true
}

// deprecationDate reads the value of Deprecation: "@<seconds>" (a structured date), an HTTP date, or "true" and anything
// else, which carry no date.
func deprecationDate(v string) time.Time {
	if seconds, ok := strings.CutPrefix(v, "@"); ok {
		if n, err := strconv.ParseInt(seconds, 10, 64); err == nil {
			return time.Unix(n, 0).UTC()
		}
		return time.Time{}
	}
	if t, err := http.ParseTime(v); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

type link struct{ url, rel string }

// links reads the target and relation of each link of Link header values: <url>; rel="x"; type="y", <url2>; rel=z.
func links(values []string) []link {
	var out []link
	for _, value := range values {
		for _, part := range splitLinks(value) {
			start, end := strings.Index(part, "<"), strings.Index(part, ">")
			if start < 0 || end < start {
				continue
			}
			l := link{url: part[start+1 : end]}
			for _, param := range strings.Split(part[end+1:], ";") {
				if name, v, ok := strings.Cut(strings.TrimSpace(param), "="); ok && strings.EqualFold(name, "rel") {
					l.rel = strings.ToLower(strings.Trim(v, `"`))
				}
			}
			out = append(out, l)
		}
	}
	return out
}

// splitLinks cuts a Link value at the commas between links, not those inside <...> or quotes.
func splitLinks(value string) []string {
	var parts []string
	depth, quoted, from := 0, false, 0
	for i, r := range value {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case r == '<':
			depth++
		case r == '>':
			depth--
		case r == ',' && depth == 0:
			parts = append(parts, value[from:i])
			from = i + 1
		}
	}
	return append(parts, value[from:])
}

// Describe says it in a line: what was announced, how far off the sunset is (or how long ago it was), and where to read more.
func (n Notice) Describe(now time.Time) string {
	var said []string
	switch {
	case n.Deprecated && !n.Since.IsZero():
		said = append(said, "deprecated since "+n.Since.Format(time.DateOnly))
	case n.Deprecated:
		said = append(said, "deprecated")
	}
	if !n.Sunset.IsZero() {
		days := int(n.Sunset.Sub(now).Hours() / 24)
		if n.Sunset.After(now) {
			said = append(said, fmt.Sprintf("will be removed on %s (in %d days)", n.Sunset.Format(time.DateOnly), days))
		} else {
			said = append(said, fmt.Sprintf("was to be removed on %s (%d days ago)", n.Sunset.Format(time.DateOnly), -days))
		}
	}
	text := strings.Join(said, ", ")
	if n.DocsURL != "" {
		text += "; see " + n.DocsURL
	}
	if n.SuccessorURL != "" {
		text += "; use " + n.SuccessorURL + " instead"
	}
	return text
}
