// SPDX-License-Identifier: Apache-2.0

package template

import "testing"

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "x", "{{a}}", "{{ a ?? 'b' }}", `{"a":"{{$int(1,2)}}"}`, "{{", "}}", "{{2s}}", "{{x == empty}}"} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, src string) {
		tpl, err := Parse(src)
		if err != nil {
			return
		}
		s := Scope{Lenient: true, Gen: func(string, []any) (any, error) { return "g", nil }}
		_, _ = tpl.Render(s) // must not panic; errors are fine for arbitrary input
		_, _ = RenderJSON(src, s)
	})
}
