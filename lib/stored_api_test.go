// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// sendAndPin sends auth/login once (which saves the response) and pins it.
func sendAndPin(t *testing.T, w *Workspace) RequestTarget {
	t.Helper()
	target, _ := w.FindRequest("auth/login", "")
	s, _ := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	res := drain(s)
	if _, err := w.PinExample(*res.Record, "ok"); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestStoredResponses_should_list_pinned_examples_and_saved_responses(t *testing.T) {
	w := demoWorkspace(t)
	target := sendAndPin(t, w)

	got, err := w.StoredResponses(target)

	if err != nil || len(got) < 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	var last, pinned *StoredResponse
	for i := range got {
		switch {
		case got[i].Kind == StoredSaved && got[i].Name == "last":
			last = &got[i]
		case got[i].Kind == StoredExample && got[i].Name == "ok":
			pinned = &got[i]
		}
	}
	if pinned == nil || pinned.Status != 200 || got[0].Kind != StoredExample {
		t.Errorf("the pinned example %+v, and examples come first: %+v", pinned, got)
	}
	if last == nil || last.Status != 200 || last.Version == "" || last.At.IsZero() {
		t.Errorf("the saved last response: %+v in %+v", last, got)
	}
}

func TestDeleteStoredResponse_should_remove_one_and_leave_the_rest(t *testing.T) {
	w := demoWorkspace(t)
	target := sendAndPin(t, w)
	before, _ := w.StoredResponses(target)
	var saved StoredResponse
	for _, s := range before {
		if s.Kind == StoredSaved && s.Name == "last" {
			saved = s
		}
	}

	errSaved := w.DeleteStoredResponse(target, StoredSaved, saved.Version, "last")
	errAgain := w.DeleteStoredResponse(target, StoredSaved, saved.Version, "last")
	errExample := w.DeleteStoredResponse(target, StoredExample, "", "ok")
	after, _ := w.StoredResponses(target)

	if errSaved != nil || errAgain != nil || errExample != nil {
		t.Fatalf("errors %v %v %v", errSaved, errAgain, errExample)
	}
	for _, s := range after {
		if s.Name == "last" || s.Name == "ok" {
			t.Errorf("%+v is still there", s)
		}
	}
	if len(after) != len(before)-2 {
		t.Errorf("before %d, after %d", len(before), len(after))
	}
}

func TestDeleteStoredResponse_Given_a_name_that_reaches_outside_Then_refuses_and_deletes_nothing(t *testing.T) {
	w := demoWorkspace(t)
	target := sendAndPin(t, w)
	outside := filepath.Join(w.Collections()[0].Root, "flowi.yaml")

	for _, name := range []string{"../../../../flowi", "../flowi", "a/b", ".hidden"} {
		if err := w.DeleteStoredResponse(target, StoredExample, "", name); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
	errKind := w.DeleteStoredResponse(target, "elsewhere", "1", "ok")
	errVersion := w.DeleteStoredResponse(target, StoredSaved, "../..", "last")

	if _, err := os.Stat(outside); err != nil || errKind == nil || errVersion == nil {
		t.Fatalf("manifest %v, kind %v, version %v", err, errKind, errVersion)
	}
}

func TestStoredRecord_should_read_a_saved_response_back(t *testing.T) {
	w := demoWorkspace(t)
	target := sendAndPin(t, w)
	list, _ := w.StoredResponses(target)
	var version string
	for _, s := range list {
		if s.Kind == StoredSaved {
			version = s.Version
		}
	}

	rec, err := w.StoredRecord(target, version, "last")

	if err != nil || rec.Status != 200 {
		t.Fatalf("record %+v, %v", rec.Status, err)
	}
	if _, err := w.StoredRecord(target, version, "missing"); err == nil {
		t.Fatal("a missing response was read")
	}
}
