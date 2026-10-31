// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCustom_should_give_the_palette_of_the_custom_theme_and_the_reason_when_it_cannot(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(good, []byte(`{"base":"light","colors":{"primary":"#268bd2"},"rainbow":["#ffd6d6"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(`{"colors":{"primary":"red"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		in          Appearance
		wantCustom  bool
		wantProblem bool
	}{
		"good":            {Appearance{Theme: "custom", paletteFile: good}, true, false},
		"bad file":        {Appearance{Theme: "custom", paletteFile: bad}, false, true},
		"no file":         {Appearance{Theme: "custom"}, false, true},
		"another theme":   {Appearance{Theme: "nord", paletteFile: bad}, false, false},
		"missing palette": {Appearance{Theme: "custom", paletteFile: filepath.Join(dir, "nope.json")}, false, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := c.in

			loadCustom(&a)

			if (a.Custom != nil) != c.wantCustom || (a.CustomProblem != "") != c.wantProblem {
				t.Fatalf("custom %+v problem %q", a.Custom, a.CustomProblem)
			}
		})
	}
}
