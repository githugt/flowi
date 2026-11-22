// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

// ImportReport is what ApplyImportReport did with the files of a plan. Every file of the plan
// is in exactly one list, so the counts always add up to the plan and a screen can say what
// happened to each request instead of "N files".
//
// Before: the result was only "Imported N files", where N was the plan's files minus the ones
// that existed when files were kept. A kept description counted as a file, a kept request counted as
// nothing, and nothing said which was which: an import that wrote one request and kept the
// document read "2 files" and looked like it had lost a request.
type ImportReport struct {
	Written   []PlannedFile // new files
	Replaced  []PlannedFile // existing files overwritten with different content
	Unchanged []PlannedFile // existing files that already held exactly this
	Kept      []PlannedFile // existing files left as they were (ApplyOptions.SkipExisting)
	Removed   []PlannedFile // tool-owned files the import replaced by removing them
}

// reportOf sorts the plan's files by what applying changes with o does to them. changes are the
// plan's files as read from disk just now, in the plan's order.
func reportOf(p ImportPlan, changes []store.FileChange, o ApplyOptions) ImportReport {
	var r ImportReport
	for i, f := range p.Files {
		ch := changes[i]
		switch {
		case ch.Old != nil && o.SkipExisting && !o.Overwrite:
			// What is kept is the file on disk, so it is named by what it holds, not by what the import would have made of it.
			if f.Kind == PlannedRequest {
				f.Name = nameOfRequestFile(f.Rel, ch.Old)
			}
			r.Kept = append(r.Kept, f)
		case ch.New == nil:
			r.Removed = append(r.Removed, f)
		case ch.Old == nil:
			r.Written = append(r.Written, f)
		case bytes.Equal(ch.Old, ch.New):
			r.Unchanged = append(r.Unchanged, f)
		default:
			r.Replaced = append(r.Replaced, f)
		}
	}
	return r
}

// annotate sets the Kind and Name of every planned file.
func (w *Workspace) annotate(p *ImportPlan) {
	for i := range p.Files {
		f := &p.Files[i]
		f.Kind, f.Name = plannedKindOf(f.Rel, f.Workspace), ""
		data := p.data[f.File]
		switch {
		case f.Workspace:
			f.Name = strings.TrimSuffix(path.Base(f.Rel), ".yaml")
		case f.Kind == PlannedRequest:
			f.Name = nameOfRequestFile(f.Rel, data)
			f.Verb = verbOfRequestFile(f.Rel, data)
		case f.Kind == PlannedFlow:
			f.Name = w.nameOfFlowFile(f, data)
		}
		if f.Exists && !f.Same && !f.Remove && (f.Kind == PlannedRequest || f.Kind == PlannedFlow) {
			if old, err := os.ReadFile(f.File); err == nil {
				if f.Kind == PlannedRequest {
					f.ExistingName = nameOfRequestFile(f.Rel, old)
				} else {
					f.ExistingName = w.nameOfFlowFile(f, old)
				}
			}
		}
	}
}

// plannedKindOf is what the file at rel (relative to the collection) is, by where it is and what it ends with.
func plannedKindOf(rel string, workspace bool) PlannedKind {
	top, _, _ := strings.Cut(rel, "/")
	switch {
	case workspace:
		return PlannedEnvironment
	case top == "requests" && strings.HasSuffix(rel, ".req.yaml"):
		return PlannedRequest
	case top == "requests" && strings.Contains(rel, ".schemas/"):
		return PlannedSchema
	case top == "requests" && strings.Contains(rel, ".examples/"):
		return PlannedExample
	case top == "flows" && strings.HasSuffix(rel, flowSuffix):
		return PlannedFlow
	case top == "auths":
		return PlannedAuth
	case top == "descriptions":
		return PlannedDocument
	case top == "protos":
		return PlannedProto
	}
	return PlannedOther
}

// verbOfRequestFile is the verb of the first version of the request a file holds: the method of an HTTP version, the
// protocol of a message version (MQTT, SQS …), GRPC. It is "" for a file that does not parse or a version of another kind.
func verbOfRequestFile(rel string, data []byte) string {
	if data == nil {
		return ""
	}
	req, diags := ParseRequest(data, filepath.Base(rel))
	if diags.HasErrors() || len(req.Versions) == 0 {
		return ""
	}
	v := req.Versions[0]
	switch {
	case v.Async != nil:
		return model.ProtocolVerb(v.Async.Protocol)
	case v.GRPC != nil:
		return model.ProtocolVerbGRPC
	}
	return v.Method
}

// nameOfRequestFile is the title a request file holds, else its path.
func nameOfRequestFile(rel string, data []byte) string {
	p := strings.TrimSuffix(strings.TrimPrefix(rel, "requests/"), ".req.yaml")
	if data == nil {
		return p
	}
	if req, diags := ParseRequest(data, filepath.Base(rel)); !diags.HasErrors() && req.Name != "" {
		return req.Name
	}
	return p
}

// nameOfFlowFile is the title a flow file holds, else its path.
func (w *Workspace) nameOfFlowFile(f *PlannedFile, data []byte) string {
	p := strings.TrimSuffix(strings.TrimPrefix(f.Rel, "flows/"), flowSuffix)
	if data == nil {
		return p
	}
	if flow, diags := w.store.DecodeFlowBody(f.File, p, data); !diags.HasErrors() && flow.Name != "" {
		return flow.Name
	}
	return p
}
