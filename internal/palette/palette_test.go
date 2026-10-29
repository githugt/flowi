// SPDX-License-Identifier: AGPL-3.0-or-later

package palette

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "palette.json")
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoad_should_map_the_named_colours_to_the_windows_variables_and_keep_the_base(t *testing.T) {
	// Arrange
	file := write(t, `{"base":"light","colors":{"background":"#fdf6e3","foreground":"#073642","primary":"#268bd2","danger":"#dc322f"},"rainbow":["#ffd6d6","#d6ffd6"]}`)

	// Act
	p, err := Load(file)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if p.Base != "light" || p.Vars["--bg"] != "#fdf6e3" || p.Vars["--text"] != "#073642" || p.Vars["--accent"] != "#268bd2" || p.Vars["--fail"] != "#dc322f" {
		t.Errorf("palette %+v", p)
	}
	if _, set := p.Vars["--panel"]; set {
		t.Error("a colour the file does not name falls back to the base theme: it must not be set here")
	}
	if len(p.Rainbow) != 2 {
		t.Errorf("rainbow %v", p.Rainbow)
	}
}

func TestLoad_Given_no_base_Then_dark(t *testing.T) {
	p, err := Load(write(t, `{"colors":{"primary":"#6ea8fe"}}`))

	if err != nil || p.Base != "dark" {
		t.Fatalf("base %q err %v", p.Base, err)
	}
}

func TestLoad_should_accept_short_hex_and_normalise_it(t *testing.T) {
	p, err := Load(write(t, `{"colors":{"primary":"#08f"}}`))

	if err != nil || p.Vars["--accent"] != "#0088ff" {
		t.Fatalf("accent %q err %v", p.Vars["--accent"], err)
	}
}

func TestLoad_should_refuse_what_cannot_be_a_theme_and_say_why(t *testing.T) {
	cases := map[string]struct {
		text string
		want string
	}{
		"not json":            {`{"colors":`, "not valid JSON"},
		"bad base":            {`{"base":"blue","colors":{}}`, "base"},
		"bad colour":          {`{"colors":{"primary":"red"}}`, "primary"},
		"css injection":       {`{"colors":{"primary":"#fff; background: url(x)"}}`, "primary"},
		"unknown colour name": {`{"colors":{"sparkle":"#fff"}}`, "sparkle"},
		"unreadable text":     {`{"base":"dark","colors":{"background":"#222222","foreground":"#2a2a2a"}}`, "contrast"},
		"bad rainbow":         {`{"rainbow":["#fff","nope"]}`, "rainbow"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, c.text))

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestLoad_Given_a_missing_file_Then_names_it(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))

	if err == nil || !strings.Contains(err.Error(), "nope.json") {
		t.Fatalf("err %v", err)
	}
}

func TestLoad_Given_only_one_of_background_and_foreground_Then_judges_against_the_base_theme(t *testing.T) {
	// A dark base has a near-white text: a white background under it would be unreadable.
	_, err := Load(write(t, `{"base":"dark","colors":{"background":"#ffffff"}}`))

	if err == nil || !strings.Contains(err.Error(), "contrast") {
		t.Fatalf("err %v", err)
	}
}
