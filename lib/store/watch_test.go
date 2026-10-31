// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const watchDeadline = 2 * time.Second

func startWatch(t *testing.T, root string) <-chan Change {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch, err := Watch(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func next(t *testing.T, ch <-chan Change) Change {
	t.Helper()
	select {
	case c, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return c
	case <-time.After(watchDeadline):
		t.Fatal("no change event before deadline")
	}
	return Change{}
}

func resolvedRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(copyTree(t, "basic"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWatch_should_emit_modified_for_request_file_write(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	file := filepath.Join(root, "requests", "auth", "login.req.yaml")

	writeFile(t, file, "id: x\n")

	got := next(t, ch)
	if got.File != file || got.Root != root || got.Op == ChangeRemoved {
		t.Fatalf("got %+v", got)
	}
}

func TestWatch_should_watch_directories_created_after_start(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	dir := filepath.Join(root, "flows", "new")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "x.arazzo.yaml")

	// The new directory is added asynchronously; keep touching the file
	// until an event for it arrives or the deadline passes.
	deadline := time.After(watchDeadline)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case c := <-ch:
			if c.File == file {
				return
			}
		case <-tick.C:
			writeFile(t, file, "id: x\n")
		case <-deadline:
			t.Fatal("no event for file in new directory")
		}
	}
}

func TestWatch_should_ignore_unrelated_files(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	request := filepath.Join(root, "requests", "auth", "login.req.yaml")

	writeFile(t, filepath.Join(root, "notes.md"), "hello\n")
	writeFile(t, filepath.Join(root, "flows", "journeys", "signup.layout.yaml"), "x: 1\n")
	writeFile(t, request, "id: y\n")

	got := next(t, ch)
	if got.File != request {
		t.Fatalf("first event %+v, want the request", got)
	}
}

func TestWatch_should_coalesce_rename_save_into_single_event(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	file := filepath.Join(root, "requests", "auth", "login.req.yaml")
	tmp := file + ".tmp"

	writeFile(t, tmp, "id: z\n")
	if err := os.Rename(tmp, file); err != nil {
		t.Fatal(err)
	}

	got := next(t, ch)
	if got.File != file || got.Op == ChangeRemoved {
		t.Fatalf("got %+v", got)
	}
	select {
	case extra := <-ch:
		t.Fatalf("unexpected second event %+v", extra)
	case <-time.After(3 * coalesceWindow):
	}
}

func TestWatch_should_close_channel_on_context_cancel(t *testing.T) {
	root := resolvedRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := Watch(ctx, []string{root})
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received an event instead of close")
		}
	case <-time.After(watchDeadline):
		t.Fatal("channel not closed")
	}
}

// collect gathers change events until want is satisfied or the deadline.
func collect(t *testing.T, ch <-chan Change, want func(map[string]ChangeOp) bool) map[string]ChangeOp {
	t.Helper()
	got := map[string]ChangeOp{}
	deadline := time.After(watchDeadline)
	for !want(got) {
		select {
		case c := <-ch:
			got[c.File] = c.Op
		case <-deadline:
			t.Fatalf("deadline passed; got %v", got)
		}
	}
	return got
}

func TestWatch_should_report_files_of_a_directory_created_with_content(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	staging := filepath.Join(t.TempDir(), "new")
	writeFile(t, filepath.Join(staging, "n.req.yaml"), "id: x\n")
	target := filepath.Join(root, "requests", "new")

	if err := os.Rename(staging, target); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(target, "n.req.yaml")
	collect(t, ch, func(got map[string]ChangeOp) bool { return got[file] == ChangeCreated })
}

func TestWatch_should_report_removal_of_a_renamed_directory(t *testing.T) {
	root := resolvedRoot(t)
	ch := startWatch(t, root)
	oldDir := filepath.Join(root, "requests", "auth")
	newDir := filepath.Join(root, "requests", "login")

	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}

	moved := filepath.Join(newDir, "login.req.yaml")
	collect(t, ch, func(got map[string]ChangeOp) bool {
		return got[oldDir] == ChangeRemoved && got[moved] == ChangeCreated
	})
}
