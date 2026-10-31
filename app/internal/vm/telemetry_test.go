// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/telemetry"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func guiClient() telemetry.Client {
	return telemetry.Client{App: "gui", Version: "1.0.0", OS: "darwin", Arch: "arm64"}
}

func TestTelemetry_Load_should_read_the_current_level_and_default_to_off(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{}
	tel := vm.NewTelemetry(b, w, guiClient())
	if tel.Level() != "off" {
		t.Fatalf("level before load %q", tel.Level())
	}
	b.Values = map[string]any{"telemetry": "minimal"}

	tel.Load()
	q.run(1)

	if tel.Level() != "minimal" {
		t.Fatalf("level %q", tel.Level())
	}
}

func TestTelemetry_Preview_should_show_the_sample_of_the_chosen_level(t *testing.T) {
	_, w := newUIQueue()
	tel := vm.NewTelemetry(&vmtest.Backend{}, w, guiClient())

	minimal, full, off := tel.Preview("minimal"), tel.Preview("full"), tel.Preview("off")

	if !json.Valid([]byte(full)) || !strings.Contains(full, `"command_run"`) || strings.Contains(minimal, "command_run") || !strings.Contains(minimal, `"app": "gui"`) {
		t.Fatalf("minimal:\n%s\nfull:\n%s", minimal, full)
	}
	if strings.Contains(off, "app_started") {
		t.Fatalf("off previews events:\n%s", off)
	}
}

func TestTelemetry_Choose_should_store_the_level_in_the_user_settings(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{}
	tel := vm.NewTelemetry(b, w, guiClient())
	changed := 0
	tel.OnChange(func() { changed++ })

	tel.Choose("full")
	q.run(1)

	if tel.Level() != "full" || changed == 0 || !b.Called("SetUserSetting telemetry=full") {
		t.Fatalf("level %q changed %d calls %v", tel.Level(), changed, b.Calls)
	}
}

func TestTelemetry_Choose_Given_a_refusal_Then_the_level_is_kept_and_the_problem_shown(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{}
	tel := vm.NewTelemetry(b, w, guiClient())

	tel.Choose("everything")
	q.run(1)

	if tel.Level() != "off" || tel.Problem() == "" {
		t.Fatalf("level %q problem %q", tel.Level(), tel.Problem())
	}
}
