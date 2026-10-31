// SPDX-License-Identifier: Apache-2.0

package diag

import "strings"

// Catalogue renders diagnostics as human-readable text.
type Catalogue interface {
	Message(d Diagnostic) (message, hint string)
}

type englishCatalogue struct{ table map[Code]codeInfo }

// English returns the default English catalogue.
func English() Catalogue {
	return englishCatalogue{table: codeTable()}
}

func (c englishCatalogue) Message(d Diagnostic) (string, string) {
	info, ok := c.table[d.Code]
	if !ok {
		return string(d.Code), ""
	}
	text := substitute(info.english, d.Args)
	message, hint, _ := strings.Cut(text, hintMarker)
	return message, hint
}

func substitute(template string, args map[string]string) string {
	pairs := make([]string, 0, 2*len(args))
	for k, v := range args {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}
