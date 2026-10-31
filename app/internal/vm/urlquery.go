// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// URLWithQuery is what the URL bar shows: the request's URL followed by its
// query parameters. Parameters without a name are not shown (a new empty
// row in the Params panel does not touch the URL).
func URLWithQuery(base string, query []model.KeyValue) string {
	var pairs []string
	for _, q := range query {
		if q.Key != "" {
			pairs = append(pairs, q.Key+"="+q.Value)
		}
	}
	if len(pairs) == 0 {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + strings.Join(pairs, "&")
}

// SplitURL is the inverse of URLWithQuery for text typed in the URL bar: the
// part before the first ? (outside {{ }}) is the URL, the rest becomes the
// query parameters. A parameter that was in old keeps its description. A
// fragment stays with the last parameter's value.
func SplitURL(text string, old []model.KeyValue) (string, []model.KeyValue) {
	at := indexOutsideTemplate(text, '?')
	if at < 0 {
		return text, nil
	}
	var query []model.KeyValue
	for _, pair := range splitOutsideTemplate(text[at+1:], '&') {
		if pair == "" {
			continue
		}
		key, value := pair, ""
		if eq := indexOutsideTemplate(pair, '='); eq >= 0 {
			key, value = pair[:eq], pair[eq+1:]
		}
		query = append(query, model.KeyValue{Key: key, Value: value, Description: describedAs(old, key)})
	}
	return text[:at], query
}

func describedAs(list []model.KeyValue, key string) string {
	for _, kv := range list {
		if kv.Key == key {
			return kv.Description
		}
	}
	return ""
}

// indexOutsideTemplate is the index of the first c in s that is not inside
// {{ }} (or a quoted string within it), or -1.
func indexOutsideTemplate(s string, c byte) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		switch {
		case depth > 0 && quote != 0:
			switch s[i] {
			case '\\':
				i++ // skip the escaped character
			case quote:
				quote = 0
			}
		case depth > 0 && (s[i] == '"' || s[i] == '\''):
			quote = s[i]
		case strings.HasPrefix(s[i:], "{{"):
			depth++
			i++
		case depth > 0 && strings.HasPrefix(s[i:], "}}"):
			depth--
			i++
		case depth == 0 && s[i] == c:
			return i
		}
	}
	return -1
}

// splitOutsideTemplate splits s at every c that is not inside {{ }}.
func splitOutsideTemplate(s string, c byte) []string {
	var out []string
	for {
		at := indexOutsideTemplate(s, c)
		if at < 0 {
			return append(out, s)
		}
		out, s = append(out, s[:at]), s[at+1:]
	}
}
