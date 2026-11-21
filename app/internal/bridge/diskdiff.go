// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"

	"github.com/githugt/flowi/lib/modeldiff"

	"github.com/githugt/flowi/app/internal/vm"
)

// DiskChangeView is one difference between the file on disk and the editor: where, how, and both values as text.
type DiskChangeView struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // added, removed or changed, from the point of view of "mine": added is only in the editor
	Disk string `json:"disk"`
	Mine string `json:"mine"`
}

// DiskDiffView is what the "This file changed on disk" banner shows under Compare.
type DiskDiffView struct {
	Changes []DiskChangeView `json:"changes"`
	More    int              `json:"more"`   // changes found and not listed
	Hidden  int              `json:"hidden"` // credential values left out
}

func diskDiffView(r modeldiff.Result) DiskDiffView {
	v := DiskDiffView{Changes: []DiskChangeView{}, More: r.More, Hidden: r.Hidden}
	for _, c := range r.Changes {
		// The comparison calls the disk "old": a value only on disk is "removed" in the editor, one only in the editor "added".
		v.Changes = append(v.Changes, DiskChangeView{Path: c.Path, Kind: string(c.Kind), Disk: c.Disk, Mine: c.Mine})
	}
	return v
}

// CompareDisk lists what differs between the file on disk and the edits in the editor.
func (s *RequestService) CompareDisk(node string) (DiskDiffView, error) {
	ch := make(chan modeldiff.Result, 1)
	s.with(node, func(o *openRequest) { ch <- o.req.DiskDiff() })
	select {
	case r := <-ch:
		return diskDiffView(r), nil
	default:
		return DiskDiffView{}, errors.New("this request is not open")
	}
}

// CompareDisk lists what differs between the flow file on disk and the flow being edited.
func (s *FlowService) CompareDisk(node string) (DiskDiffView, error) {
	ch := make(chan modeldiff.Result, 1)
	s.with(node, func(d *vm.FlowDoc) { ch <- d.DiskDiff() })
	select {
	case r := <-ch:
		return diskDiffView(r), nil
	default:
		return DiskDiffView{}, errors.New("this flow is not open")
	}
}
