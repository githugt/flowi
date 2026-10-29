// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi-lib"
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
