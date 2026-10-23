// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/template"
)

// Generator is a $function the suggestions can offer.
type Generator struct {
	Name   string // without the $
	Detail string
}

// SuggestSource is what a template can read where the user is typing.
type SuggestSource struct {
	Env        []string // variables of the selected environment
	Secrets    []string
	Vars       []string // the request's own variables ({{var.x}})
	Generators []Generator
}

// Suggestion is one thing to insert: Insert is the expression without
// braces ("env.base_url", "$uuid-v4").
type Suggestion struct {
	Insert string
	Detail string
}

// Completion is the suggestions for the template being typed. From and To
// are the byte range of the typed expression (after "{{" and any spaces,
// up to the caret).
type Completion struct {
	From, To int
	Items    []Suggestion
}

// Suggest lists up to limit suggestions for the template open before caret
// (a byte index into text): functions after "{{$", else the variables of
// the namespace typed so far, all of them after a bare "{{". Nothing when
// the caret is not inside an unclosed "{{".
func Suggest(text string, caret int, src SuggestSource, limit int) Completion {
	caret = min(max(caret, 0), len(text))
	before := text[:caret]
	open := strings.LastIndex(before, "{{")
	if open < 0 || strings.Contains(before[open:], "}}") {
		return Completion{}
	}
	from := open + 2
	for from < caret && before[from] == ' ' {
		from++
	}
	typed := before[from:]
	var items []Suggestion
	for _, c := range candidates(typed, src) {
		if strings.HasPrefix(strings.ToLower(c.Insert), strings.ToLower(typed)) {
			items = append(items, c)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return Completion{From: from, To: caret, Items: items}
}

// candidates are what could follow the typed part: functions after a $,
// otherwise every namespace's names (each sorted).
func candidates(typed string, src SuggestSource) []Suggestion {
	if strings.HasPrefix(typed, "$") {
		out := make([]Suggestion, 0, len(src.Generators))
		for _, g := range src.Generators {
			out = append(out, Suggestion{Insert: "$" + g.Name, Detail: g.Detail})
		}
		slices.SortFunc(out, func(a, b Suggestion) int { return strings.Compare(a.Insert, b.Insert) })
		return out
	}
	var out []Suggestion
	for _, ns := range []struct {
		prefix string
		names  []string
		detail string
	}{{"env.", src.Env, "environment"}, {"var.", src.Vars, "request variable"}, {"secret.", src.Secrets, "secret"}} {
		for _, n := range slices.Sorted(slices.Values(ns.names)) {
			out = append(out, Suggestion{Insert: ns.prefix + n, Detail: ns.detail})
		}
	}
	return out
}

// BuiltinGenerators are the $functions the template engine offers.
func BuiltinGenerators() []Generator {
	var out []Generator
	for _, g := range template.Generators() {
		out = append(out, Generator{Name: g.Name, Detail: strings.TrimSpace(g.Args + " " + g.Description)})
	}
	return out
}

// RequestSource is the suggestion source for a request of a collection:
// the environment's variables and secrets, the request's own variables and
// the built-in functions.
func RequestSource(envVars, secrets []string, v []string) SuggestSource {
	return SuggestSource{Env: envVars, Secrets: secrets, Vars: v, Generators: BuiltinGenerators()}
}
