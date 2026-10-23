// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
)

// What the Data tab of the flow editor shows for a node after a run, as plain
// parts the view only has to draw.

// PartKind is how a Part is drawn.
type PartKind int

// The kinds of part.
const (
	PartLines PartKind = iota // a few lines of text
	PartTable                 // name/value rows
	PartCode                  // text in the code view
)

// Part is one block of a node's Input or Output.
type Part struct {
	Heading   string
	Kind      PartKind
	Lines     []string
	Rows      []model.KeyValue
	Code      string
	Lang      string // the code's language (LangJSON, LangPlain)
	Collapsed bool   // a long table shown closed
}

// ViewFormat is how a response body is shown.
type ViewFormat int

// The formats of a body.
const (
	ViewJSON ViewFormat = iota // pretty-printed, when the body parsed as JSON
	ViewText                   // as text, when it is text
	ViewRaw                    // as it is, or what it is when it is not text
)

// bodyShowLimit is how much of a body's text is shown: more would only make
// the tab slow for nothing anyone reads.
const bodyShowLimit = 256 * 1024

// RenderBody is body b in format f. ok is false when the body has no such form
// (JSON for a body that did not parse, text for a binary one).
func RenderBody(b engine.Body, f ViewFormat) (text, lang string, ok bool) {
	switch f {
	case ViewJSON:
		if b.JSON == nil {
			return "", "", false
		}
		pretty, err := json.MarshalIndent(b.JSON, "", "  ")
		if err != nil {
			return "", "", false
		}
		return capText(string(pretty)), LangJSON, true
	case ViewText:
		if b.Text == "" && b.Base64 != "" {
			return "", "", false
		}
		return capText(b.Text), DetectLanguage(b.ContentType, b.Text), true
	}
	if b.Text == "" && b.Base64 != "" {
		return fmt.Sprintf("<%d bytes of %s>", b.Size, valueOr(b.ContentType, "unknown type")), LangPlain, true
	}
	return capText(b.Text), LangPlain, true
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// capText cuts text at bodyShowLimit and says how much was left out.
func capText(s string) string {
	if len(s) <= bodyShowLimit {
		return s
	}
	return s[:bodyShowLimit] + fmt.Sprintf("\n… %d more bytes left out", len(s)-bodyShowLimit)
}

// InputParts is what went into node n in the last run: for a request node the
// request as sent; for the others their parameters as written (the editor does
// not have the evaluated values). Nil before the node ran.
func InputParts(n model.Node, d NodeDetail) []Part {
	if d.Status == "" {
		return nil
	}
	if rec := d.Record; rec != nil {
		return sentParts(*rec)
	}
	var parts []Part
	if n.Request != nil {
		parts = append(parts, Part{Heading: "Request", Kind: PartLines, Lines: []string{n.Request.Request.Path}})
		if rows := n.Request.With; len(rows) > 0 {
			parts = append(parts, Part{Heading: "With (as written)", Kind: PartTable, Rows: rows})
		}
		return parts
	}
	if len(n.Params) == 0 {
		return nil
	}
	rows := make([]model.KeyValue, 0, len(n.Params))
	for _, f := range n.Params {
		rows = append(rows, model.KeyValue{Key: f.Key, Value: fmt.Sprint(f.Value)})
	}
	return []Part{{Heading: "Parameters (as written)", Kind: PartTable, Rows: rows}}
}

func sentParts(rec engine.Record) []Part {
	parts := []Part{{Kind: PartLines, Lines: []string{rec.Request.Method + " " + rec.Request.URL}}}
	if len(rec.Request.Header) > 0 {
		parts = append(parts, Part{Heading: "Headers", Kind: PartTable, Rows: headerRows(rec.Request.Header)})
	}
	if rec.HasMessages() {
		for _, m := range rec.Messages {
			if m.Direction == "sent" {
				text, lang, _ := RenderBody(m.Body, ViewJSON)
				if text == "" {
					text, lang, _ = RenderBody(m.Body, ViewRaw)
				}
				parts = append(parts, Part{Heading: "Message sent", Kind: PartCode, Code: text, Lang: lang})
			}
		}
		return parts
	}
	if b := rec.Request.Body; b != nil {
		text, lang, ok := RenderBody(*b, ViewJSON)
		if !ok {
			text, lang, _ = RenderBody(*b, ViewRaw)
		}
		if text != "" {
			parts = append(parts, Part{Heading: "Body", Kind: PartCode, Code: text, Lang: lang})
		}
	}
	return parts
}

// OutputParts is what node n produced in the last run: for a request node the
// response (status, headers, body in format f, timings, extracted values); for
// the others their outputs. Nil when there is nothing.
func OutputParts(_ model.Node, d NodeDetail, f ViewFormat) []Part {
	if rec := d.Record; rec != nil {
		return receivedParts(*rec, f)
	}
	if len(d.Outputs) == 0 {
		return nil
	}
	pretty, err := json.MarshalIndent(d.Outputs, "", "  ")
	if err != nil {
		return nil
	}
	return []Part{{Heading: "Outputs", Kind: PartCode, Code: string(pretty), Lang: LangJSON}}
}

func receivedParts(rec engine.Record, f ViewFormat) []Part {
	parts := []Part{{Kind: PartLines, Lines: []string{statusLine(rec)}}}
	if len(rec.Header) > 0 {
		parts = append(parts, Part{Heading: "Headers", Kind: PartTable, Rows: headerRows(rec.Header), Collapsed: true})
	}
	if rec.HasMessages() {
		for _, m := range rec.Messages {
			if m.Direction != "sent" {
				text, lang, ok := RenderBody(m.Body, f)
				if !ok {
					text, lang, _ = RenderBody(m.Body, ViewRaw)
				}
				parts = append(parts, Part{Heading: "Message received", Kind: PartCode, Code: text, Lang: lang})
			}
		}
	} else {
		text, lang, ok := RenderBody(rec.Body, f)
		if !ok {
			text, lang, ok = RenderBody(rec.Body, ViewRaw)
		}
		if ok && text != "" {
			parts = append(parts, Part{Heading: "Body", Kind: PartCode, Code: text, Lang: lang})
		}
	}
	if len(rec.Outputs) > 0 {
		parts = append(parts, Part{Heading: "Extracted", Kind: PartTable, Rows: sortedRows(rec.Outputs)})
	}
	if lines := timingLines(rec.Timings); len(lines) > 0 {
		parts = append(parts, Part{Heading: "Timings", Kind: PartLines, Lines: lines})
	}
	return parts
}

// statusLine is "200 · 84ms", or the outcome of a message-based call.
func statusLine(rec engine.Record) string {
	total := rec.Timings.Total.Round(time.Millisecond)
	switch {
	case rec.GRPCCode != "":
		return fmt.Sprintf("%s · %s", rec.GRPCCode, total)
	case rec.HasMessages():
		return fmt.Sprintf("%s · %d messages", rec.Outcome, len(rec.Messages))
	}
	return fmt.Sprintf("%d · %s", rec.Status, total)
}

func timingLines(t engine.Timings) []string {
	var lines []string
	for _, p := range []struct {
		name string
		d    time.Duration
	}{{"dns", t.DNS}, {"connect", t.Connect}, {"tls", t.TLS}, {"ttfb", t.TTFB}, {"total", t.Total}} {
		if p.d > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", p.name, p.d.Round(time.Millisecond)))
		}
	}
	return lines
}

func headerRows(hs []engine.Header) []model.KeyValue {
	rows := make([]model.KeyValue, 0, len(hs))
	for _, h := range hs {
		rows = append(rows, model.KeyValue{Key: h.Name, Value: h.Value})
	}
	return rows
}

func sortedRows(m map[string]any) []model.KeyValue {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]model.KeyValue, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, model.KeyValue{Key: k, Value: fmt.Sprint(m[k])})
	}
	return rows
}

// TestRow is one check of a node in the last run.
type TestRow struct {
	Name, Detail string
	Passed       bool
}

// assertFailedPrefix starts a diagnostic line of a failed assertion
// (diag.CodeRunAssertFailed, as FlowRun formats diagnostics).
const assertFailedPrefix = "FLW-RUN-001"

// TestRows are the checks of node n: an assert node is one row; a request node
// has a failed row for each assertion that failed (the engine reports passes
// only as the node's status). Other nodes have none.
func TestRows(n model.Node, d NodeDetail) []TestRow {
	if d.Status == "" {
		return nil
	}
	switch {
	case n.Type == "assert":
		check := fmt.Sprint(valueOr(fmt.Sprint(paramOf(n, "check")), n.ID))
		row := TestRow{Name: check, Passed: d.Status == "passed"}
		if !row.Passed {
			row.Detail = strings.Join(d.Diags, "\n")
		}
		return []TestRow{row}
	case n.Request != nil:
		var rows []TestRow
		for _, line := range d.Diags {
			if strings.HasPrefix(line, assertFailedPrefix) {
				msg := strings.TrimPrefix(strings.TrimPrefix(line, assertFailedPrefix+":"), " ")
				rows = append(rows, TestRow{Name: strings.TrimPrefix(msg, "assertion failed: "), Detail: line})
			}
		}
		return rows
	}
	return nil
}

func paramOf(n model.Node, key string) any {
	for _, f := range n.Params {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}
