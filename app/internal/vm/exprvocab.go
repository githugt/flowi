// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "strings"

// exprFunctions are the functions an expression can call, offered with their bracket open.
func exprFunctions() []Suggestion {
	return []Suggestion{
		{"len(", "how many items or characters"}, {"count(", "how many items pass a test"},
		{"map(", "work on every item of a list"}, {"filter(", "keep the items that pass a test"},
		{"any(", "does any item pass"}, {"all(", "do all items pass"},
		{"upper(", "upper case"}, {"lower(", "lower case"}, {"trim(", "cut spaces at both ends"},
		{"split(", "cut text into a list"}, {"join(", "join a list into text"}, {"replace(", "replace text"},
		{"hasPrefix(", "starts with"}, {"hasSuffix(", "ends with"},
		{"int(", "convert to a whole number"}, {"float(", "convert to a decimal number"},
		{"string(", "convert to text"}, {"toJSON(", "value as JSON text"}, {"fromJSON(", "JSON text as a value"},
		{"sum(", "add up a list of numbers"}, {"mean(", "average of a list"}, {"max(", "largest number"}, {"min(", "smallest number"},
		{"abs(", "distance from zero"}, {"round(", "nearest whole number"},
		{"first(", "first item of a list"}, {"last(", "last item of a list"}, {"sort(", "put a list in order"}, {"uniq(", "drop repeated items"},
		{"concat(", "join lists"}, {"keys(", "the names of a map"}, {"values(", "the values of a map"},
		{"now(", "the current time"}, {"date(", "read a date or date-time"}, {"duration(", "read a length of time such as 90s"},
		{"toBase64(", "encode text as Base64"}, {"fromBase64(", "decode Base64 text"},
	}
}

// exprKeywords are the words an expression reads besides names and functions.
func exprKeywords() []Suggestion {
	return []Suggestion{
		{"empty", "null, an empty text, list or map"}, {"contains", "text contains text"},
		{"matches", "text matches a pattern"}, {"startsWith", "text starts with text"}, {"endsWith", "text ends with text"},
		{"and", "both tests are true"}, {"or", "either test is true"}, {"not", "the test is false"},
		{"in", "is one of a list"}, {"true", ""}, {"false", ""}, {"null", ""},
	}
}

// ExpressionVocabulary lists what can start a word in an expression written in ctx: the names that
// context can read (up to a dot where the rest is the user's to choose, as in "nodes."), then the
// functions and keywords. The page filters it as the user types.
func ExpressionVocabulary(ctx ExprContext) []Suggestion {
	var out []Suggestion
	seen := map[string]bool{}
	add := func(s Suggestion) {
		if s.Insert == "" || seen[s.Insert] {
			return
		}
		seen[s.Insert] = true
		out = append(out, s)
	}
	for _, item := range ExpressionHelp(ctx).Names {
		for _, name := range strings.Split(item.Name, ", ") {
			if root, ok := nameRoot(name); ok {
				add(Suggestion{Insert: root, Detail: item.What})
			}
		}
	}
	for _, s := range exprFunctions() {
		add(s)
	}
	for _, s := range exprKeywords() {
		add(s)
	}
	return out
}

// nameRoot turns a help name into what is typed: "body.<field>" and "response.*" stop at the dot
// ("body."), and a generator, which only templates take, is left out.
func nameRoot(name string) (string, bool) {
	if strings.HasPrefix(name, "$") {
		return "", false
	}
	if i := strings.IndexAny(name, "<*"); i >= 0 {
		name = name[:i]
	}
	return name, name != ""
}
