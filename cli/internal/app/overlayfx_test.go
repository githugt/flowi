// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func writeOverlay(t *testing.T, e *sendEnv, name, body string) {
	t.Helper()
	writeFile(t, filepath.Join(e.dir, "tools", "overlays", name+".flowi-overlay.yaml"), "flowi_overlay: 1\nname: "+name+"\n"+body)
}

func TestOverlayShow_Given_for_Then_it_lists_what_the_overlay_changes_and_says_when_it_is_an_identity(t *testing.T) {
	e := newSendEnv(t)
	writeOverlay(t, e, "ann", "shared:\n  - set: headers.Authorization\n    value: Bearer ann\n")
	writeOverlay(t, e, "slow", "shared:\n  - set: headers.X-Delay\n    value: 2s\n")

	code, out, errOut := e.run(context.Background(), "overlay", "show", "ann", "--collection", "Tools", "--for", "slow")
	_, plain, _ := e.run(context.Background(), "overlay", "show", "slow", "--collection", "Tools", "--for", "slow")

	if code != 0 || !strings.Contains(out, "Authorization") || !strings.Contains(out, "identity") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if strings.Contains(plain, "identity") || !strings.Contains(plain, "X-Delay") {
		t.Fatalf("slow:\n%s", plain)
	}
}
