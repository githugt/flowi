// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequestActivity_Given_a_request_made_here_Then_it_says_created(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, "mine/one")
	if err != nil {
		t.Fatal(err)
	}

	a, err := w.RequestActivity(target)

	if err != nil || a.Origin != OriginCreated || a.Created.IsZero() || a.Runs != 0 {
		t.Fatalf("activity %+v %v", a, err)
	}
}

func TestRequestActivity_Given_a_request_that_came_with_the_files_Then_this_client_starts_its_record(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target := RequestTarget{Collection: c.Manifest.ID, Request: c.Requests[0].ID}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(c.Requests[0].File, old, old); err != nil {
		t.Fatal(err)
	}

	first, err := w.RequestActivity(target)
	second, _ := w.RequestActivity(target)

	if err != nil || first.Origin != OriginFound || !first.Created.Equal(old.Truncate(0)) && first.Created.Sub(old).Abs() > time.Second {
		t.Fatalf("first look %+v %v", first, err)
	}
	if !second.Created.Equal(first.Created) {
		t.Fatalf("the record changed between looks: %+v then %+v", first, second)
	}
}

func TestRequestActivity_Given_a_damaged_record_Then_it_is_started_again_not_an_error(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target := RequestTarget{Collection: c.Manifest.ID, Request: c.Requests[0].ID}
	file := w.activityFile(target.Collection, activityRequest, target.Request)
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	_ = os.WriteFile(file, []byte("{not json"), 0o644)

	a, err := w.RequestActivity(target)

	if err != nil || a.Origin != OriginFound {
		t.Fatalf("activity %+v %v", a, err)
	}
}

func TestSendRequest_Then_the_last_run_and_last_success_are_kept(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target := RequestTarget{Collection: c.Manifest.ID, Request: c.Requests[0].ID}

	w.noteRun(target.Collection, activityRequest, target.Request, "passed", c.Requests[0].File)
	w.noteRun(target.Collection, activityRequest, target.Request, "failed", c.Requests[0].File)
	a, _ := w.RequestActivity(target)

	if a.Runs != 2 || a.LastStatus != "failed" || a.LastSuccess.IsZero() || a.LastFailure.IsZero() || a.LastRun.Before(a.LastSuccess) {
		t.Fatalf("activity %+v", a)
	}
}

func TestDuplicateRequest_Then_the_copy_says_what_it_is_a_copy_of(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	copied, err := w.DuplicateRequest(RequestTarget{Collection: c.Manifest.ID, Request: c.Requests[0].ID}, "dup/copy")
	if err != nil {
		t.Fatal(err)
	}

	a, _ := w.RequestActivity(copied)

	if a.Origin != OriginDuplicated || a.OriginDetail != "copy of "+c.Requests[0].Path {
		t.Fatalf("activity %+v", a)
	}
}

func TestFlowActivity_Given_a_flow_run_Then_the_run_is_counted(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, err := w.NewFlow(c.Manifest.ID, "mine/flow")
	if err != nil {
		t.Fatal(err)
	}

	run, err := w.RunFlow(context.Background(), target, RunOptions{})
	if err == nil {
		for range run.Events() {
		}
		run.Wait()
	}
	deadline := time.Now().Add(2 * time.Second)
	var a Activity
	for time.Now().Before(deadline) {
		a, _ = w.FlowActivity(target)
		if a.Runs > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err == nil && a.Runs != 1 {
		t.Fatalf("activity %+v", a)
	}
	if a.Origin != OriginCreated {
		t.Fatalf("origin %+v", a)
	}
}
