// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"strings"
	"testing"
)

func TestEnglish_should_have_template_for_every_code(t *testing.T) {
	cat := English()

	for _, c := range Codes() {
		msg, _ := cat.Message(New(c, Location{}, nil))

		if msg == "" {
			t.Errorf("no English message for %s", c)
		}
	}
}

func TestEnglish_should_substitute_args(t *testing.T) {
	d := New(CodeRefUnknownRequest, Location{}, map[string]string{"id": "01JABCDEF"})

	msg, _ := English().Message(d)

	if !strings.Contains(msg, "01JABCDEF") {
		t.Fatalf("message %q does not contain the id", msg)
	}
}

func TestEnglish_should_split_hint_after_marker(t *testing.T) {
	d := New(CodeWSMissing, Location{}, map[string]string{"id": "01J", "path": "/x"})

	msg, hint := English().Message(d)

	if hint != `run "flowi workspace locate"` {
		t.Fatalf("hint %q", hint)
	}
	if strings.Contains(msg, "hint") || strings.Contains(msg, "locate") {
		t.Fatalf("message %q still contains the hint", msg)
	}
}

func TestEnglish_should_render_M2_secret_hint_with_command(t *testing.T) {
	d := New(CodeSecretMissing, Location{}, map[string]string{"name": "password", "env": "staging"})

	msg, hint := English().Message(d)

	if msg != `secret "password" is not set for environment "staging"` {
		t.Fatalf("message %q", msg)
	}
	if hint != "set it with `flowi secret set staging password`" {
		t.Fatalf("hint %q", hint)
	}
}
