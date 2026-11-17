// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

func TestServeFlags_Given_fault_flags_Then_they_reach_the_serve_options(t *testing.T) {
	// Arrange
	cmd := newServeCommand(nil)
	args := []string{"--fault-error", "0.25", "--fault-status", "502", "--fault-drop", "0.1", "--fault-jitter", "150ms"}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	var f serveFlags
	f.faultError, f.faultStatus, f.faultDrop, f.faultJitter = 0.25, 502, 0.1, 150*time.Millisecond

	// Act
	got := f.options(cmd, flowi.ServeOptions{})

	// Assert
	if got.Faults.ErrorRate != 0.25 || got.Faults.ErrorStatus != 502 || got.Faults.DropRate != 0.1 || got.Faults.Jitter != 150*time.Millisecond {
		t.Fatalf("faults %+v", got.Faults)
	}
}

func TestServeFlags_Given_forward_flags_Then_they_reach_the_serve_options(t *testing.T) {
	cmd := newServeCommand(nil)
	if err := cmd.ParseFlags([]string{"--forward", "save", "--forward-only", "https://a.test,https://b.test/v1", "--forward-env", "staging", "--record-to", "shop"}); err != nil {
		t.Fatal(err)
	}
	f := serveFlags{forward: "save", forwardOnly: []string{"https://a.test", "https://b.test/v1"}, forwardEnv: "staging", recordTo: "shop"}

	got := f.options(cmd, flowi.ServeOptions{})

	if got.Forward != flowi.ForwardSave || len(got.ForwardOnly) != 2 || got.ForwardOnly[1] != "https://b.test/v1" || got.ForwardEnv != "staging" || got.RecordTo != "shop" || got.ForwardTo != "" {
		t.Fatalf("options %+v", got)
	}
}
