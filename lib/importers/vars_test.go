// SPDX-License-Identifier: Apache-2.0

package importers

import "testing"

func TestTranslate_should_map_variables_and_dynamic_variables(t *testing.T) {
	cases := map[string]string{
		"{{base_url}}/v1":            "{{env.base_url}}/v1",
		"{{base url}}":               `{{env["base url"]}}`,
		"id {{$guid}}":               "id {{$uuid-v4}}",
		"{{$randomInt}}":             "{{$int(0, 1000)}}",
		"{{$isoTimestamp}}":          "{{$now}}",
		"{{ spaced }}":               "{{env.spaced}}",
		"open {{ never closed":       `open {{ "{{" }} never closed`,
		"{{$randomBankAccountIban}}": "{{env.randomBankAccountIban}}",
		"{{process.env.API_HOST}}":   "{{env.API_HOST}}",
		"Bearer {{token}}":           "Bearer {{secret.token}}",
	}
	for in, want := range cases {
		got, _ := Translate(in)
		if got != want {
			t.Errorf("Translate(%q) = %q, want %q", in, got, want)
		}
	}
	if _, unknown := Translate("{{$randomBankAccountIban}}"); len(unknown) != 1 || unknown[0] != "$randomBankAccountIban" {
		t.Fatalf("unknown dynamic variables are reported: %v", unknown)
	}
}
