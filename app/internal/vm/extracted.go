// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"fmt"

	"github.com/githugt/flowi/lib/model"
)

// extractedMax is how much of an extracted value a chip shows.
const extractedMax = 60

// Extracted is what one of a request's extractions took from a response.
type Extracted struct {
	Name  string
	Value string // the value as text, cut for a chip; "" when nothing matched
	Found bool
}

// ExtractNames are the names of the extractions a send will try, in order; nil when there are none.
func ExtractNames(rows []model.Extraction) []string {
	var out []string
	for _, e := range rows {
		if e.Name != "" {
			out = append(out, e.Name)
		}
	}
	return out
}

// ExtractedOf says, for each extraction the exchange tried, what it took from the response or that nothing matched.
// An extraction added or renamed after the send is not in the list: it has not been tried.
func ExtractedOf(ex Exchange) []Extracted {
	out := make([]Extracted, 0, len(ex.Extracts))
	for _, name := range ex.Extracts {
		item := Extracted{Name: name}
		if rec := ex.Result.Record; rec != nil {
			if v, ok := rec.Outputs[name]; ok {
				item.Found, item.Value = true, shortenValue(v)
			}
		}
		out = append(out, item)
	}
	return out
}

func shortenValue(v any) string {
	text, ok := v.(string)
	if !ok {
		b, err := json.Marshal(v)
		if err != nil {
			text = fmt.Sprint(v)
		} else {
			text = string(b)
		}
	}
	if r := []rune(text); len(r) > extractedMax {
		return string(r[:extractedMax]) + "…"
	}
	return text
}
