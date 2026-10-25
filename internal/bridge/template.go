// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"unicode/utf16"

	"github.com/githugt/flowi/internal/vm"
)

// SpanView is a run of a URL or template, with what it is for colouring
// (scheme, host, path, variable, unknown-variable, key, value…).
type SpanView struct {
	Text string `json:"text"`
	Part string `json:"part"`
	Pair int    `json:"pair"` // the key=value pair it belongs to, or -1
}

// SuggestionView is one thing a template can be completed with.
type SuggestionView struct {
	Insert string `json:"insert"`
	Detail string `json:"detail"`
}

// SuggestView is the suggestions for the template being typed. From and To
// are UTF-16 indices into the text (what the page counts in).
type SuggestView struct {
	From  int              `json:"from"`
	To    int              `json:"to"`
	Items []SuggestionView `json:"items"`
}

// utf16Index turns a byte offset of text into a UTF-16 index.
func utf16Index(text string, byteOffset int) int {
	byteOffset = min(max(byteOffset, 0), len(text))
	n := 0
	for _, r := range text[:byteOffset] {
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}

// byteOffsetOf turns a UTF-16 index into a byte offset of text.
func byteOffsetOf(text string, index int) int {
	n := 0
	for i, r := range text {
		if n >= index {
			return i
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return len(text)
}

// URLSpans colours a URL as typed: parts of the URL and which variables
// are defined (for the selected environment, and the request's own).
func (s *RequestService) URLSpans(node, text string) []SpanView {
	out := []SpanView{}
	s.with(node, func(*openRequest) {
		vars, secrets := s.app.workspace.VariableNames()
		known := vm.NewNameChecker(vars, secrets)
		for _, sp := range vm.TokenizeURL(text, known) {
			out = append(out, SpanView{Text: sp.Text, Part: sp.Part.String(), Pair: sp.Pair})
		}
	})
	return out
}

// Suggest offers what the template open before caret (a UTF-16 index) can
// be completed with: environment variables, secrets, the request's own
// variables and the built-in functions.
func (s *RequestService) Suggest(node, text string, caret int) SuggestView {
	v := SuggestView{Items: []SuggestionView{}}
	s.with(node, func(o *openRequest) {
		vars, secrets := s.app.workspace.VariableNames()
		var own []string
		for _, kv := range o.req.Current().Variables {
			if kv.Key != "" {
				own = append(own, kv.Key)
			}
		}
		c := vm.Suggest(text, byteOffsetOf(text, caret), vm.RequestSource(vars, secrets, own), s.suggestMax())
		v.From, v.To = utf16Index(text, c.From), utf16Index(text, c.To)
		for _, it := range c.Items {
			v.Items = append(v.Items, SuggestionView{Insert: it.Insert, Detail: it.Detail})
		}
	})
	return v
}

// suggestMax is ui.suggestions_max.
func (s *RequestService) suggestMax() int {
	if v, ok := s.app.backend.Settings("").Get("ui.suggestions_max"); ok {
		if n, isInt := v.Value.(int64); isInt {
			return int(n)
		}
	}
	return 3
}

// Complete offers what a template in the selected step can read (flow
// inputs, environment keys, secrets, outputs of earlier steps), for the
// template open before caret (a UTF-16 index).
func (s *FlowService) Complete(node, text string, caret int) SuggestView {
	v := SuggestView{Items: []SuggestionView{}}
	_ = s.edit(node, func(o *openFlow) error {
		path := ""
		if o.selected != "" {
			path = o.doc.NodePath(o.selected)
		}
		all, err := s.app.backend.Completions(o.doc.Collection().Manifest.ID, o.doc.Draft(), path)
		if err != nil {
			return nil // no completions is no problem for the field
		}
		byteCaret := byteOffsetOf(text, caret)
		matches, start := vm.Complete(text, byteCaret, all)
		v.From, v.To = utf16Index(text, start), utf16Index(text, byteCaret)
		for _, m := range matches {
			v.Items = append(v.Items, SuggestionView{Insert: m.Text, Detail: m.Detail})
		}
		return nil
	})
	return v
}

// ExpressionVocabulary lists what can start a word in an expression written in the given
// context (see vm.ExprContext): the names that context reads, functions and keywords. The page
// filters it as the user types, so no round trip is needed per key.
func (s *SystemService) ExpressionVocabulary(context string) []SuggestionView {
	out := []SuggestionView{}
	for _, it := range vm.ExpressionVocabulary(vm.ExprContext(context)) {
		out = append(out, SuggestionView{Insert: it.Insert, Detail: it.Detail})
	}
	return out
}

// ExpressionNames lists the names an expression in the selected step can read, written out in
// full: flow inputs, environment keys, secrets, and the outputs of earlier steps, including body
// fields known from their pinned examples or last saved responses.
func (s *FlowService) ExpressionNames(node string) []SuggestionView {
	out := []SuggestionView{}
	_ = s.edit(node, func(o *openFlow) error {
		path := ""
		if o.selected != "" {
			path = o.doc.NodePath(o.selected)
		}
		all, err := s.app.backend.Completions(o.doc.Collection().Manifest.ID, o.doc.Draft(), path)
		if err != nil {
			return nil // no names is no problem for the field
		}
		for _, c := range all {
			out = append(out, SuggestionView{Insert: c.Text, Detail: c.Detail})
		}
		return nil
	})
	return out
}

// ExpressionNames lists the environment variables and the request's own variables an expression
// can read, written out in full ("env.base_url", "var.page").
func (s *RequestService) ExpressionNames(node string) []SuggestionView {
	out := []SuggestionView{}
	s.with(node, func(o *openRequest) {
		vars, _ := s.app.workspace.VariableNames()
		for _, n := range slices.Sorted(slices.Values(vars)) {
			out = append(out, SuggestionView{Insert: "env." + n, Detail: "environment"})
		}
		for _, kv := range o.req.Current().Variables {
			if kv.Key != "" {
				out = append(out, SuggestionView{Insert: "var." + kv.Key, Detail: "request variable"})
			}
		}
		// What the request is known to return, from its schema, examples or last saved response: no
		// send is needed to offer body.data.id.
		if fields, err := s.app.backend.ResponseFields(o.req.Target(), o.req.SelectedLabel()); err == nil {
			for _, f := range fields {
				out = append(out, SuggestionView{Insert: f.Text, Detail: f.Detail})
			}
		}
	})
	return out
}
