// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import "testing"

// Before: colour could only be turned off (NO_COLOR) and never forced, and the output format had to be repeated on
// every command line.
func TestConsole_UseColor_should_let_the_flag_override_NO_COLOR(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		mode    string
		want    bool
	}{
		{"auto, nothing set", nil, ColorAuto, true},
		{"auto, NO_COLOR", []string{"NO_COLOR=1"}, ColorAuto, false},
		{"always beats NO_COLOR", []string{"NO_COLOR=1"}, ColorAlways, true},
		{"never", nil, ColorNever, false},
		{"empty mode is auto", nil, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConsole(nil, nil, nil, tt.environ, nil)

			got := c.UseColor(tt.mode)

			if got != tt.want {
				t.Fatalf("UseColor(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGlobalOptions_validate_should_refuse_an_unknown_colour_mode(t *testing.T) {
	good := &GlobalOptions{Output: OutputText, Color: ColorNever}
	bad := &GlobalOptions{Output: OutputText, Color: "rainbow"}

	if err := good.validate(); err != nil {
		t.Fatalf("never: %v", err)
	}
	if err := bad.validate(); err == nil {
		t.Fatal("an unknown colour mode must be a usage error")
	}
}

func TestGlobalOptions_applyEnv_should_use_FLOWI_OUTPUT_only_when_the_flag_was_not_given(t *testing.T) {
	environ := []string{"FLOWI_OUTPUT=json"}
	fromEnv := &GlobalOptions{Output: OutputText}
	fromFlag := &GlobalOptions{Output: OutputGitHub}
	empty := &GlobalOptions{Output: OutputText}

	fromEnv.applyEnv(environ, false)
	fromFlag.applyEnv(environ, true)
	empty.applyEnv([]string{"FLOWI_OUTPUT="}, false)

	if fromEnv.Output != OutputJSON {
		t.Errorf("env: output %q, want json", fromEnv.Output)
	}
	if fromFlag.Output != OutputGitHub {
		t.Errorf("flag: output %q, want github (the flag wins)", fromFlag.Output)
	}
	if empty.Output != OutputText {
		t.Errorf("empty env: output %q, want text", empty.Output)
	}
}
