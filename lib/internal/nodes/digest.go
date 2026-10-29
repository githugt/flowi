// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

func digestSpec() Spec {
	return Spec{
		Type: "digest", Group: "Output", Summary: "A summary of the run so far, or of a finished node, sent to stdout, a file or a webhook.",
		Params: []Param{
			{Name: "of", Kind: ParamString, Doc: `"run" (default) or the id of an earlier node.`},
			{Name: "sink", Kind: ParamEnum, Enum: []string{"stdout", "file", "webhook"}, Doc: "Default stdout."},
			{Name: "format", Kind: ParamEnum, Enum: []string{"markdown", "text", "json"}, Doc: "Default markdown."},
			{Name: "file", Kind: ParamName, Doc: "For sink file: a plain file name, written under the workspace's `digests/<flow-id>/`."},
			{Name: "url", Kind: ParamText, Doc: `For sink webhook: receives {"text": …}, or the report itself for format json.`},
			{Name: "style", Kind: ParamEnum, Enum: digestStyles(), Doc: "For sink webhook: the payload shape: text and slack send {\"text\"}, discord {\"content\"}, teams a MessageCard, email {to, subject, text}. Default text. Ignored for format json."},
			{Name: "to", Kind: ParamText, Doc: "For style email: the recipient."},
			{Name: "subject", Kind: ParamText, Doc: `For style email: the subject. Default "Flowi digest".`},
		},
		Run: runDigest,
	}
}

func runDigest(ctx context.Context, n model.Node, s *State) Outcome {
	of, format := orDefault(stringParam(n, "of"), "run"), orDefault(stringParam(n, "format"), "markdown")
	r, ok := s.Runtime.Report(of)
	if !ok {
		return failed(n, diag.CodeRunDigestTarget, map[string]string{"of": of})
	}
	text, err := formatReport(r, format)
	if err != nil {
		return Outcome{Status: StatusError, Diags: diag.List{newDiag(n, diag.CodeNodeParam, map[string]string{"field": "format", "detail": err.Error()})}}
	}
	text = s.Masker.String(text)
	switch orDefault(stringParam(n, "sink"), "stdout") {
	case "file":
		file := stringParam(n, "file")
		if !names.Valid(file) {
			return failed(n, diag.CodeRunFileName, map[string]string{"name": file})
		}
		return writeOutput(n, s, "digests", file, []byte(text), false)
	case "webhook":
		return digestWebhook(ctx, n, s, format, text)
	}
	s.Emit.Digest(n.ID, format, text)
	return Outcome{}
}

func formatReport(r report.Run, format string) (string, error) {
	writers := map[string]func(io.Writer, report.Run) error{
		"markdown": report.WriteMarkdown, "text": report.WriteText, "json": report.WriteJSON,
	}
	var buf bytes.Buffer
	if err := writers[format](&buf, r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// digestWebhook posts the digest in the shape the node's style asks for,
// or the JSON report itself.
func digestWebhook(ctx context.Context, n model.Node, s *State, format, text string) Outcome {
	raw, _ := param(n, "url")
	url, err := eval(raw, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "url", err)}
	}
	payload, out := digestPayload(n, s, format, text)
	if out.Diags.HasErrors() {
		return out
	}
	res := s.Runtime.HTTP(ctx, HTTPCall{Method: "POST", URL: stringOf(url), Body: payload})
	if res.Response != nil && res.Response.Status/100 != 2 {
		// A receiver that refuses the digest (too long, bad URL) must not look delivered.
		return failed(n, diag.CodeHTTPFailed, map[string]string{"detail": "the digest webhook answered " + strconv.Itoa(res.Response.Status)})
	}
	return Outcome{Status: res.Status, Diags: res.Diags}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

// digestStyles are the webhook payload shapes, in the order they are listed.
func digestStyles() []string { return []string{"text", "slack", "discord", "teams", "email"} }

const defaultDigestSubject = "Flowi digest"

// Message limits of the receivers, in characters (Discord answers 400 above
// its limit; the others are kept safely below theirs).
const (
	discordLimit = 2000
	slackLimit   = 39000
	teamsLimit   = 20000
	truncated    = "\n… (truncated)"
)

// clip shortens text to at most limit characters, ending with a marker.
func clip(text string, limit int) string {
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	return string(r[:limit-len([]rune(truncated))]) + truncated
}

// digestPayload builds the webhook body. The report text is already masked;
// the recipient and subject are rendered like any other template field.
func digestPayload(n model.Node, s *State, format, text string) (any, Outcome) {
	if format == "json" {
		return json.RawMessage(text), Outcome{}
	}
	style := orDefault(stringParam(n, "style"), "text")
	switch style {
	case "text":
		return map[string]any{"text": text}, Outcome{}
	case "slack":
		return map[string]any{"text": clip(text, slackLimit)}, Outcome{}
	case "discord":
		return map[string]any{"content": clip(text, discordLimit)}, Outcome{}
	case "teams":
		return map[string]any{
			"@type": "MessageCard", "@context": "https://schema.org/extensions",
			"summary": defaultDigestSubject, "text": clip(text, teamsLimit),
		}, Outcome{}
	case "email":
		return emailPayload(n, s, text)
	}
	return nil, Outcome{Status: StatusError, Diags: diag.List{newDiag(n, diag.CodeNodeParam, map[string]string{"field": "style", "detail": "unknown style " + style})}}
}

func emailPayload(n model.Node, s *State, text string) (any, Outcome) {
	to, out := renderedParam(n, s, "to")
	if out.Diags.HasErrors() {
		return nil, out
	}
	if to == "" {
		return nil, Outcome{Status: StatusError, Diags: diag.List{newDiag(n, diag.CodeNodeParam, map[string]string{"field": "to", "detail": "style email needs a recipient"})}}
	}
	subject, out := renderedParam(n, s, "subject")
	if out.Diags.HasErrors() {
		return nil, out
	}
	return map[string]any{"to": to, "subject": orDefault(s.Masker.String(subject), defaultDigestSubject), "text": text}, Outcome{}
}

// renderedParam evaluates a string parameter as a template.
func renderedParam(n model.Node, s *State, name string) (string, Outcome) {
	raw, set := param(n, name)
	if !set {
		return "", Outcome{}
	}
	v, err := eval(raw, s)
	if err != nil {
		return "", Outcome{Diags: templateDiags(n, name, err)}
	}
	return stringOf(v), Outcome{}
}
