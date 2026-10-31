// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// maxShown bounds a value in a one-line description.
const maxShown = 60

// Describe says what op does in a short sentence a user reads in a save
// dialog or a warning: "Set header X-Tier to "vip"".
func Describe(op model.OverlayOp) string {
	field, rest, _ := strings.Cut(op.Path, ".")
	noun := map[string]string{"headers": "header", "query": "query parameter", "variables": "variable", "settings": "setting", "extract": "extraction"}[field]
	switch {
	case noun != "" && op.Verb == model.OverlaySet:
		return fmt.Sprintf("Set %s %s to %s", noun, rest, shown(op.Value))
	case noun != "" && op.Verb == model.OverlayRemove:
		return fmt.Sprintf("Remove %s %s", noun, rest)
	case field == "assert" && op.Verb == model.OverlayAdd:
		return "Add check " + shown(op.Value)
	case field == "assert" && op.Verb == model.OverlayRemove:
		return "Remove check " + shown(op.Value)
	case field == "body" && op.Verb == model.OverlayMerge:
		return "Change the JSON body: " + shown(op.Value)
	case field == "body":
		return "Replace the text body with " + shown(op.Value)
	case field == "hooks" && op.Verb == model.OverlayAdd:
		return fmt.Sprintf("Run the flow %s as a %s hook", hookName(op.Value), rest)
	case field == "hooks" && op.Verb == model.OverlayRemove:
		return fmt.Sprintf("Do not run the flow %s as a %s hook", hookName(op.Value), rest)
	case op.Verb == model.OverlayRemove:
		return fmt.Sprintf("Clear the %s", labels(field))
	case op.Verb == model.OverlaySet:
		return fmt.Sprintf("Set the %s to %s", labels(field), shown(op.Value))
	}
	return op.Verb + " " + op.Path
}

func labels(field string) string {
	if field == "docs" {
		return "documentation"
	}
	return field
}

func shown(v any) string {
	var s string
	switch t := v.(type) {
	case string:
		s = `"` + t + `"`
	default:
		if data, err := json.Marshal(normalise(v)); err == nil {
			s = string(data)
		} else {
			s = fmt.Sprint(v)
		}
	}
	if r := []rune(s); len(r) > maxShown {
		return string(r[:maxShown-1]) + "…"
	}
	return s
}

// hookName names the flow of a hook operation: its path hint when it has one.
func hookName(value any) string {
	ref, err := hookRef(value)
	if err != nil {
		return shown(value)
	}
	if ref.Path != "" {
		return `"` + ref.Path + `"`
	}
	return `"` + string(ref.ID) + `"`
}
