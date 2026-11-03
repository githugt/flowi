// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
)

// Masked stands in for the value of a credential header in an Exchange.
const Masked = "••••"

// CaptureLimit is the most of a body a Capture keeps; the rest is cut and the Message says so.
const CaptureLimit = 64 << 10

// Message is one side of a call: the headers (credential values masked) and the body as text.
type Message struct {
	Header    http.Header
	Body      string // text; "(binary, N bytes)" for what is not
	Truncated bool   // the body was longer than CaptureLimit
}

// Exchange is what a call carried and what it was answered with, kept when Options.Capture is set. It stays in the
// process: the Entry's JSON, which /__flowi/traffic serves, leaves it out.
type Exchange struct {
	Sent Message
	Got  Message
}

// maskedHeader copies h with the values of credential headers replaced.
func maskedHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for name, values := range h {
		if engine.LooksLikeCredential(name) {
			out[name] = []string{Masked}
			continue
		}
		out[name] = append([]string(nil), values...)
	}
	return out
}

// bodyText turns the first CaptureLimit bytes of a body into what is shown for it.
func textOfBody(b []byte, total int) (text string, truncated bool) {
	truncated = total > len(b)
	if bytes.IndexByte(b, 0) >= 0 || !utf8.Valid(b) && !truncated {
		return fmt.Sprintf("(binary, %d bytes)", total), false
	}
	return string(b), truncated
}

// captureRequest reads the request body for the Exchange and puts it back, so the handler still sees all of it.
func captureRequest(r *http.Request) Message {
	var kept []byte
	var total int
	if r.Body != nil && r.Body != http.NoBody {
		all, _ := io.ReadAll(io.LimitReader(r.Body, CaptureLimit+1)) // a read error just shortens what is kept
		total = len(all)
		kept = all[:min(len(all), CaptureLimit)]
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(all), r.Body))
	}
	text, cut := textOfBody(kept, total)
	return Message{Header: maskedHeader(r.Header), Body: text, Truncated: cut}
}

// captureWriter passes an answer through and keeps its first CaptureLimit bytes.
type captureWriter struct {
	http.ResponseWriter
	body  bytes.Buffer
	total int
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if room := CaptureLimit - c.body.Len(); room > 0 {
		c.body.Write(p[:min(len(p), room)])
	}
	c.total += len(p)
	return c.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the real writer.
func (c *captureWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

func (c *captureWriter) message() Message {
	text, cut := textOfBody(c.body.Bytes(), c.total)
	return Message{Header: maskedHeader(c.Header()), Body: text, Truncated: cut}
}
