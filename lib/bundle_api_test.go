// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/bundle"
	"github.com/githugt/flowi-lib/internal/tapeplayer"
)

func runToEnd(t *testing.T, w *Workspace, target FlowTarget, o RunOptions) engine.RunResult {
	t.Helper()
	run, err := w.RunFlow(context.Background(), target, o)
	if err != nil {
		t.Fatal(err)
	}
	for range run.Events() { //nolint:revive // draining the events lets the run end
	}
	return run.Wait()
}

func recordedBundle(t *testing.T) ([]byte, engine.RunResult) {
	t.Helper()
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)
	opts := RunOptions{Env: "local", Seed: 7}
	res := runToEnd(t, w, target, opts)
	if res.Status != engine.StatusPassed {
		t.Fatalf("the original run: %s %v", res.Status, res.Diags)
	}
	var buf bytes.Buffer
	if err := w.WriteBundle(&buf, target, opts, res); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), res
}

func TestOpenBundle_Given_a_bundle_in_a_workspace_with_nothing_else_Then_the_run_plays_back_offline_with_the_same_outcome(t *testing.T) {
	// Arrange
	data, original := recordedBundle(t)
	other := newEnv(t).open(t) // no collection, no environment, no vault, no server

	// Act
	br, err := other.OpenBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	replay := runToEnd(t, other, br.Target, br.Options)

	// Assert
	if replay.Status != original.Status || replay.Seed != original.Seed {
		t.Fatalf("replay %s seed %d, original %s seed %d\n%v", replay.Status, replay.Seed, original.Status, original.Seed, replay.Diags)
	}
	if len(replay.Report.Nodes) != len(original.Report.Nodes) || len(replay.Captured) != len(original.Captured) {
		t.Fatalf("nodes %d/%d captured %d/%d", len(replay.Report.Nodes), len(original.Report.Nodes), len(replay.Captured), len(original.Captured))
	}
}

func TestOpenBundle_Given_it_is_closed_Then_the_workspace_has_no_trace_of_it(t *testing.T) {
	// Arrange
	data, _ := recordedBundle(t)
	other := newEnv(t).open(t)
	br, err := other.OpenBundle(data)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	br.Close()

	// Assert
	if len(other.Collections()) != 0 {
		t.Fatalf("collections left: %d", len(other.Collections()))
	}
}

func TestWriteBundle_Then_no_secret_value_and_no_run_products_are_inside(t *testing.T) {
	// Arrange
	data, _ := recordedBundle(t)

	// Act
	b, err := bundle.Read(data)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if b.Manifest.Secrets == nil || b.Manifest.Env != "local" || b.Manifest.Seed != 7 {
		t.Errorf("manifest %+v", b.Manifest)
	}
	if !strings.Contains(string(readEntry(t, data, "sequence.mmd")), "sequenceDiagram") {
		t.Errorf("the bundle should carry the sequence diagram of the run")
	}
	for name := range b.Files {
		if strings.Contains(name, ".examples/") || strings.Contains(name, ".schemas/") || strings.HasPrefix(name, "descriptions/") {
			t.Errorf("a run product or a derived file is in the bundle: %s", name)
		}
	}
	if strings.Contains(string(b.Environment), "demo") && strings.Contains(string(b.Environment), "password: demo") {
		t.Errorf("a secret value reached the environment file:\n%s", b.Environment)
	}
}

func TestOpenBundle_Given_a_call_the_recording_never_made_Then_the_replay_says_so(t *testing.T) {
	// Arrange
	data, _ := recordedBundle(t)
	b, _ := bundle.Read(data)
	b.Records = b.Records[:1] // the later calls are gone
	var cut bytes.Buffer
	if err := bundle.Write(&cut, b); err != nil {
		t.Fatal(err)
	}
	other := newEnv(t).open(t)
	br, err := other.OpenBundle(cut.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()

	// Act
	replay := runToEnd(t, other, br.Target, br.Options)

	// Assert
	if replay.Status == engine.StatusPassed || !strings.Contains(diagText(replay), tapeplayer.ErrNotRecorded.Error()) {
		t.Fatalf("status %s diags %v", replay.Status, replay.Diags)
	}
}

func diagText(r engine.RunResult) string {
	var sb strings.Builder
	for _, d := range r.Diags {
		msg, _ := diag.English().Message(d)
		sb.WriteString(msg + "\n")
	}
	for _, n := range r.Report.Nodes {
		for _, d := range n.Diagnostics {
			sb.WriteString(d.Message + "\n")
		}
	}
	return sb.String()
}

func TestOpenBundle_Given_the_original_collection_is_mounted_in_the_same_workspace_Then_the_copy_gets_new_ids_and_plays_back(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)
	opts := RunOptions{Env: "local", Seed: 3}
	original := runToEnd(t, w, target, opts)
	var buf bytes.Buffer
	if err := w.WriteBundle(&buf, target, opts, original); err != nil {
		t.Fatal(err)
	}

	// Act
	br, err := w.OpenBundle(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	replay := runToEnd(t, w, br.Target, br.Options)

	// Assert
	if br.Target.Collection == target.Collection || br.Target.Flow == target.Flow {
		t.Errorf("the copy must not share ids with the original: %+v / %+v", br.Target, target)
	}
	if replay.Status != engine.StatusPassed {
		t.Fatalf("replay %s %v", replay.Status, replay.Diags)
	}
}

func TestKeepBundle_Given_a_run_started_the_ordinary_way_in_a_bundles_collection_Then_it_is_a_replay(t *testing.T) {
	// Arrange: an app run button passes the tab's environment and no seed; the bundle decides both.
	data, original := recordedBundle(t)
	other := newEnv(t).open(t)
	opened, err := other.KeepBundle(data)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	replay := runToEnd(t, other, opened.Target, RunOptions{Env: "staging"})
	again := runToEnd(t, other, opened.Target, RunOptions{})

	// Assert
	if replay.Status != original.Status || replay.Seed != original.Seed {
		t.Fatalf("replay %s seed %d; original %s seed %d\n%v", replay.Status, replay.Seed, original.Status, original.Seed, replay.Diags)
	}
	if again.Status != original.Status {
		t.Errorf("a second run in the same window: %s %v", again.Status, again.Diags)
	}
	if name := other.Collections()[0].Manifest.Name; !strings.Contains(name, "replay") || !strings.Contains(name, original.Report.FlowName) && !strings.Contains(name, "User creation") {
		t.Errorf("a replay collection must not look like the original: %q", name)
	}
	if !other.IsReplayCollection(opened.Target.Collection) {
		t.Errorf("the collection should be known as a replay")
	}
}

// readEntry reads one named entry of a bundle zip.
func readEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		out, _ := io.ReadAll(rc)
		return out
	}
	t.Fatalf("no entry %s", name)
	return nil
}
