// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"fmt"
	"slices"
	"strings"
)

// Reference renders the node reference (docs/reference/nodes.md) from
// Specs: one section per group, one entry per type, in catalogue order.
func Reference() []byte {
	var b strings.Builder
	b.WriteString("# Node reference\n\n")
	b.WriteString("Generated from the node registry (`go test ./internal/nodes/ -run Reference -update`). Do not edit.\n\n")
	b.WriteString("Every node also takes the common fields `id`, `name`, `description`, `needs`, `when`, `concurrent`, `on_error` (`fail`, `continue`, `goto:<id>`), `timeout` and `attach` (the id of a request or subflow node right next to it that the canvas shows it with; a view only, the run order is the list order) and `attach_at` (`above` or `below`: draw it in a stack above or below that request, not beside it).\n")
	specs := Specs()
	for _, group := range groupsOf(specs) {
		fmt.Fprintf(&b, "\n## %s\n", group)
		for _, s := range specs {
			if s.Group == group {
				writeSpec(&b, s)
			}
		}
	}
	return []byte(b.String())
}

func groupsOf(specs []Spec) []string {
	var groups []string
	for _, s := range specs {
		if !slices.Contains(groups, s.Group) {
			groups = append(groups, s.Group)
		}
	}
	return groups
}

func writeSpec(b *strings.Builder, s Spec) {
	fmt.Fprintf(b, "\n### `%s`\n\n%s\n", s.Type, s.Summary)
	if len(s.Typed) > 0 {
		fmt.Fprintf(b, "\nFields: %s.\n", codeJoin(s.Typed))
	}
	if s.MapForm {
		b.WriteString("\nOther keys are `<variable>: template`; each result is stored in `vars.<variable>` and output as `nodes.<id>.<variable>`.\n")
	}
	if len(s.Params) == 0 {
		return
	}
	b.WriteString("\n| Parameter | Kind | Required | Notes |\n|---|---|---|---|\n")
	for _, p := range s.Params {
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", p.Name, kindName(p), yesNo(p.Required), p.Doc)
	}
}

func kindName(p Param) string {
	names := map[ParamKind]string{
		ParamString: "string", ParamText: "template", ParamExpr: "expression", ParamDuration: "duration",
		ParamInt: "integer", ParamBool: "boolean", ParamName: "file name", ParamAny: "any", ParamMap: "map of templates",
	}
	if p.Kind == ParamEnum {
		return "one of " + codeJoin(p.Enum)
	}
	if p.Kind == ParamInt && (p.Min != 0 || p.Max != 0) {
		return intRange(p)
	}
	return names[p.Kind]
}

func intRange(p Param) string {
	if p.Max == 0 {
		return fmt.Sprintf("integer ≥ %d", p.Min)
	}
	return fmt.Sprintf("integer %d–%d", p.Min, p.Max)
}

func codeJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "`" + s + "`"
	}
	return strings.Join(quoted, ", ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return ""
}
