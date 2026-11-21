// SPDX-License-Identifier: Apache-2.0

// Package bugreport writes a request and the response it got as Markdown for a bug report or a chat message, with
// every value that looks like a credential masked and a list of what was masked.
//
// Before: sharing a request with its response took three manual copies (the request, the headers, the body), each
// with whatever secrets were in it.
//
// Masking is by name, not by value: a header, query parameter or JSON field named like a credential (Authorization,
// Cookie, token, password, api_key…) loses its value, and Flowi's own secrets, already masked in a record, read
// <secret>. A secret in a field with an ordinary name stays, so the text is meant to be read before it is sent.
package bugreport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/snippet"
	"github.com/githugt/flowi/lib/vault"
)

// Options sizes the report.
type Options struct {
	// MaxBody is how many bytes of each body the report keeps; 0 means 4096.
	MaxBody int
	// MaxHeaders is how many headers of each side it lists; 0 means 25.
	MaxHeaders int
}

// Report is the Markdown and what was masked in it, one line for each kind of value: "Authorization header (request)".
type Report struct {
	Markdown string
	Masked   []string
}

const (
	defaultMaxBody    = 4096
	defaultMaxHeaders = 25
	maxHeaderValue    = 200
	placeholder       = "<secret>"
)

type builder struct {
	o      Options
	masked []string
	sb     strings.Builder
}

// Build writes the report of rec.
func Build(rec engine.Record, o Options) Report {
	if o.MaxBody <= 0 {
		o.MaxBody = defaultMaxBody
	}
	if o.MaxHeaders <= 0 {
		o.MaxHeaders = defaultMaxHeaders
	}
	b := &builder{o: o}
	url := b.url(rec.Request.URL)
	method := rec.Request.Method
	if method == "" {
		method = http.MethodGet
	}

	fmt.Fprintf(&b.sb, "### %s %s → %s\n\n", method, url, b.outcome(rec))
	b.facts(rec)
	reqHeaders := b.headers(rec.Request.Header, "request")
	b.curl(method, url, reqHeaders, rec.Request.Body)
	b.headerBlock("Request headers", reqHeaders)
	if body := rec.Request.Body; body != nil {
		b.bodyBlock("Request body", *body, "request")
	}
	if rec.HasMessages() {
		b.messages(rec)
	} else {
		b.headerBlock("Response headers", b.headers(rec.Header, "response"))
		b.bodyBlock("Response body", rec.Body, "response")
	}
	if len(b.masked) > 0 {
		fmt.Fprintf(&b.sb, "> Masked before copying: %s. Read the text before you share it: a secret in a field with an ordinary name is not masked.\n", strings.Join(b.masked, ", "))
	} else {
		b.sb.WriteString("> Nothing here looked like a credential. Read the text before you share it.\n")
	}
	return Report{Markdown: b.sb.String(), Masked: b.masked}
}

func (b *builder) note(what string) {
	for _, m := range b.masked {
		if m == what {
			return
		}
	}
	b.masked = append(b.masked, what)
}

// text hides Flowi's own secrets (already masked in a record) behind a placeholder that says what they are.
func (b *builder) text(s string) string {
	if !strings.Contains(s, vault.Mask) {
		return s
	}
	b.note("Flowi secrets")
	return strings.ReplaceAll(s, vault.Mask, placeholder)
}

func (b *builder) outcome(rec engine.Record) string {
	switch {
	case rec.HasMessages() && rec.Kind == engine.KindGRPC:
		return strings.TrimSpace("gRPC " + rec.GRPCCode)
	case rec.HasMessages():
		return rec.Outcome
	case rec.Status == 0:
		return "no response"
	}
	if t := http.StatusText(rec.Status); t != "" {
		return fmt.Sprintf("%d %s", rec.Status, t)
	}
	return fmt.Sprint(rec.Status)
}

func (b *builder) facts(rec engine.Record) {
	var facts []string
	if rec.Timings.Total > 0 {
		facts = append(facts, fmt.Sprintf("took %d ms", rec.Timings.Total.Milliseconds()))
	}
	if rec.Body.Size > 0 {
		facts = append(facts, fmt.Sprintf("%d bytes", rec.Body.Size))
	}
	if rec.Proto != "" {
		facts = append(facts, rec.Proto)
	}
	if n := len(rec.Attempts); n > 1 {
		facts = append(facts, fmt.Sprintf("%d attempts", n))
	}
	if rec.Version != "" {
		facts = append(facts, "version "+rec.Version)
	}
	if rec.Overlay != "" {
		facts = append(facts, "overlay "+rec.Overlay)
	}
	if len(facts) > 0 {
		b.sb.WriteString(strings.Join(facts, " · ") + "\n\n")
	}
}

// url masks the login and the credential-named query parameters of a URL.
func (b *builder) url(raw string) string {
	if stripped, had := importers.StripUserinfo(raw); had {
		raw = stripped
		b.note("login in the URL")
	}
	base, query, found := strings.Cut(raw, "?")
	if found {
		query, frag, hasFrag := strings.Cut(query, "#")
		parts := strings.Split(query, "&")
		for i, p := range parts {
			name, value, hasValue := strings.Cut(p, "=")
			if hasValue && value != "" && importers.SensitiveHeader(name) && !importers.HasVar(value) && !strings.Contains(value, vault.Mask) {
				parts[i] = name + "=" + importers.Redacted
				b.note(name + " query parameter")
			}
		}
		raw = base + "?" + strings.Join(parts, "&")
		if hasFrag {
			raw += "#" + frag
		}
	}
	return b.text(raw)
}

// headers returns hs with credential values replaced and the rest cut to a readable length.
func (b *builder) headers(hs []engine.Header, side string) []engine.Header {
	out := make([]engine.Header, len(hs))
	for i, h := range hs {
		switch {
		case importers.SensitiveHeader(h.Name) && !importers.HasVar(h.Value) && !strings.Contains(h.Value, vault.Mask):
			h.Value = importers.Redacted
			b.note(fmt.Sprintf("%s header (%s)", h.Name, side))
		default:
			h.Value = b.text(h.Value)
		}
		if utf8.RuneCountInString(h.Value) > maxHeaderValue {
			h.Value = string([]rune(h.Value)[:maxHeaderValue]) + "…"
		}
		out[i] = h
	}
	return out
}

func (b *builder) curl(method, url string, headers []engine.Header, body *engine.Body) {
	req := engine.PreviewRequest{Method: method, URL: url, Headers: headers}
	if body != nil {
		if text, _ := b.bodyText(*body, "request"); text != "" {
			req.BodyKind, req.Body = "text", text
		}
	}
	if len(req.Headers) > b.o.MaxHeaders {
		req.Headers = req.Headers[:b.o.MaxHeaders]
	}
	if text, err := snippet.Generate(snippet.Curl, req); err == nil {
		b.sb.WriteString("**cURL**\n\n" + fence(text, "sh") + "\n")
	}
}

func (b *builder) headerBlock(title string, hs []engine.Header) {
	if len(hs) == 0 {
		return
	}
	shown := hs
	more := ""
	if len(shown) > b.o.MaxHeaders {
		shown, more = shown[:b.o.MaxHeaders], fmt.Sprintf("\n… %d more", len(hs)-b.o.MaxHeaders)
	}
	lines := make([]string, len(shown))
	for i, h := range shown {
		lines[i] = h.Name + ": " + h.Value
	}
	b.sb.WriteString("**" + title + "**\n\n" + fence(strings.Join(lines, "\n")+more, "") + "\n")
}

// bodyText is the body as text with credential-named JSON fields masked, and how it was cut ("" when it was not).
func (b *builder) bodyText(body engine.Body, side string) (text, cut string) {
	switch {
	case body.JSON != nil:
		redacted, n := importers.RedactJSON(body.JSON)
		if n > 0 {
			b.note(fmt.Sprintf("%d JSON field%s named like a credential (%s body)", n, plural(n), side))
		}
		pretty, err := json.MarshalIndent(redacted, "", "  ")
		if err == nil {
			text = string(pretty)
		}
	case body.Text != "":
		text = body.Text
	default:
		return "", ""
	}
	text = b.text(text)
	if len(text) > b.o.MaxBody {
		n := b.o.MaxBody
		for n > 0 && !utf8.RuneStart(text[n]) {
			n--
		}
		return text[:n], fmt.Sprintf("… %d more bytes not shown", len(text)-n)
	}
	return text, ""
}

func (b *builder) bodyBlock(title string, body engine.Body, side string) {
	text, cut := b.bodyText(body, side)
	switch {
	case text != "":
		b.sb.WriteString("**" + title + "**\n\n" + fence(text, "") + "\n")
		if cut != "" {
			b.sb.WriteString("_" + cut + "_\n\n")
		}
	case body.Base64 != "" || body.File != "" || body.Size > 0:
		fmt.Fprintf(&b.sb, "**%s**\n\nBinary or file content, %d bytes, not included.\n\n", title, body.Size)
	}
}

func (b *builder) messages(rec engine.Record) {
	for i, m := range rec.Messages {
		if i == 5 {
			fmt.Fprintf(&b.sb, "_… %d more messages_\n\n", len(rec.Messages)-5)
			break
		}
		b.bodyBlock(fmt.Sprintf("Message %d (%s)", i+1, m.Direction), m.Body, "message")
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// fence wraps text in a code fence long enough that the text cannot close it.
func fence(text, lang string) string {
	ticks, run := 3, 0
	for _, r := range text {
		if r == '`' {
			run++
			if run >= ticks {
				ticks = run + 1
			}
		} else {
			run = 0
		}
	}
	f := strings.Repeat("`", ticks)
	return f + lang + "\n" + text + "\n" + f + "\n"
}
