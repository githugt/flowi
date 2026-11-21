// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/engine"
)

const (
	kilobyte = 1000
	megabyte = 1000 * kilobyte
)

// TookText is how long a send took, in the unit that reads best: "820 µs", "142 ms", "1.24 s".
// A zero duration means nothing was measured and gives "".
func TookText(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Millisecond:
		return fmt.Sprintf("%d µs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

// SizeText is a body's size as "512 B", "1.2 KB" or "3.4 MB". A negative size means unknown and gives "";
// an empty body is "0 B" because that is a fact worth showing.
func SizeText(n int64) string {
	switch {
	case n < 0:
		return ""
	case n < kilobyte:
		return fmt.Sprintf("%d B", n)
	case n < megabyte:
		return fmt.Sprintf("%.1f KB", float64(n)/kilobyte)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/megabyte)
}

// ResponseFacts is the line of facts about how a response came: the HTTP version, the compression with its size on
// the wire and decoded, and the redirects it passed through. "" when there is nothing to say (a message exchange, or a
// plain HTTP/1.1 response that was not compressed or redirected).
//
// Before: the record kept Proto, RawSize and Redirects but nothing showed them, and a 4 KB response read as 340 KB.
func ResponseFacts(rec *engine.Record) string {
	if rec == nil || rec.HasMessages() || rec.Kind != "" {
		return ""
	}
	var parts []string
	if v := protoText(rec.Proto); v != "" && v != "HTTP/1.1" {
		parts = append(parts, v)
	}
	if rec.Body.Encoding != "" && rec.Body.RawSize > 0 {
		parts = append(parts, fmt.Sprintf("%s %s → %s", rec.Body.Encoding, SizeText(rec.Body.RawSize), SizeText(rec.Body.Size)))
	}
	if n := len(rec.Redirects); n > 0 {
		parts = append(parts, plural(n, "redirect"))
	}
	return strings.Join(parts, " · ")
}

// protoText writes the protocol the way people say it: "HTTP/2", not "HTTP/2.0".
func protoText(p string) string {
	return strings.TrimSuffix(p, ".0")
}

// RedirectLines lists the hops of a response as "301 /a → /b", one for each redirect, with a note on any credential
// header that went on to another origin.
func RedirectLines(rec *engine.Record) []string {
	if rec == nil {
		return nil
	}
	out := make([]string, 0, len(rec.Redirects))
	for _, h := range rec.Redirects {
		line := fmt.Sprintf("%d %s → %s", h.Status, h.URL, h.Location)
		if len(h.Forwarded) > 0 {
			line += " (credentials forwarded: " + strings.Join(h.Forwarded, ", ") + ")"
		}
		out = append(out, line)
	}
	return out
}
