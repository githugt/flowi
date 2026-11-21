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

func TestServeFlags_Given_health_flags_Then_they_reach_the_serve_options(t *testing.T) {
	// Before: the only health route was on the trigger endpoint and could not be asked for on its own.
	cmd := newServeCommand(nil)
	if err := cmd.ParseFlags([]string{"--health-addr", "0.0.0.0:8527", "--metrics"}); err != nil {
		t.Fatal(err)
	}
	f := serveFlags{healthAddr: "0.0.0.0:8527", metrics: true}

	got := f.options(cmd, flowi.ServeOptions{})
	off := serveFlags{}.options(newServeCommand(nil), flowi.ServeOptions{})

	if got.HealthAddr != "0.0.0.0:8527" || !got.Metrics {
		t.Fatalf("options %+v", got)
	}
	if off.HealthAddr != "" || off.Metrics {
		t.Fatalf("health is off by default: %+v", off)
	}
}

func TestServeFlags_Given_tls_flags_Then_they_reach_the_serve_options(t *testing.T) {
	// Before: the trigger endpoint could not serve HTTPS, and a wide bind only produced a note.
	cmd := newServeCommand(nil)
	if err := cmd.ParseFlags([]string{"--tls-cert", "c.pem", "--tls-key", "k.pem", "--insecure-listen"}); err != nil {
		t.Fatal(err)
	}
	f := serveFlags{tlsCert: "c.pem", tlsKey: "k.pem", insecureListen: true}

	got := f.options(cmd, flowi.ServeOptions{})

	if got.TLSCert != "c.pem" || got.TLSKey != "k.pem" || !got.InsecureListen {
		t.Fatalf("options %+v", got)
	}
}
