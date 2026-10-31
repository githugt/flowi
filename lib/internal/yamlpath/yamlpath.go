// SPDX-License-Identifier: Apache-2.0

// Package yamlpath builds YAML paths ("$.nodes[1].request.id") in the
// syntax goccy/go-yaml's path lookup understands. The codec and the
// validator must agree on it, so it lives in one place.
package yamlpath

import (
	"strconv"
	"strings"
)

// Key appends a mapping key. Keys made only of letters, digits, '_' and
// '-' are written bare; others are single-quoted with quotes escaped.
func Key(parent, key string) string {
	if isPlain(key) {
		return parent + "." + key
	}
	return parent + ".'" + strings.ReplaceAll(key, "'", `\'`) + "'"
}

// Index appends a sequence index.
func Index(parent string, i int) string {
	return parent + "[" + strconv.Itoa(i) + "]"
}

func isPlain(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		plain := r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !plain {
			return false
		}
	}
	return true
}
