// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: the banner for a file that changed on disk offered Load or Keep and nothing to choose by.
func TestRequestVM_Given_the_file_changed_under_edits_Then_the_diff_lists_theirs_and_mine(t *testing.T) {
	b := sampleBackend()
	r, _ := newRequestVM(t, b)
	disk := b.Cols[0].Requests[0]
	disk.Versions = slices.Clone(disk.Versions)
	disk.Versions[0].URL = "https://theirs.example"

	r.Edit(func(v *model.RequestVersion) { v.Method = "PUT" })
	r.Reloaded(b.Cols[0], disk, true)
	d := r.DiskDiff()

	var paths []string
	for _, c := range d.Changes {
		paths = append(paths, c.Path)
	}
	if !r.ChangedOnDisk() || len(d.Changes) != 2 || !slices.Contains(paths, "$.Versions[0].URL") || !slices.Contains(paths, "$.Versions[0].Method") {
		t.Fatalf("changes %+v", d.Changes)
	}
	for _, c := range d.Changes {
		if c.Path == "$.Versions[0].URL" && !strings.Contains(c.Disk, "theirs") {
			t.Fatalf("url change %+v", c)
		}
		if c.Path == "$.Versions[0].Method" && !strings.Contains(c.Mine, "PUT") {
			t.Fatalf("method change %+v", c)
		}
	}
}

func TestRequestVM_Given_no_edits_Then_the_diff_is_empty(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	if d := r.DiskDiff(); !d.Same() {
		t.Fatalf("changes %+v", d.Changes)
	}
}

func TestFlowDoc_Given_the_file_changed_under_edits_Then_the_diff_names_the_changed_fields(t *testing.T) {
	d, b, _ := newFlowDoc(t)
	disk := b.Cols[0].Flows[0]
	disk.Description = "changed elsewhere"

	d.Delete("a")
	d.Reloaded(b.Cols[0], disk, true)
	diff := d.DiskDiff()

	var described bool
	for _, c := range diff.Changes {
		described = described || (c.Path == "$.Description" && strings.Contains(c.Disk, "changed elsewhere"))
	}
	if !described || len(diff.Changes) < 2 {
		t.Fatalf("changes %+v", diff.Changes)
	}
}
