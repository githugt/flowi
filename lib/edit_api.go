// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/version"
)

// StoreEdit replaces the request's one dirty edit with v, started from the
// version labelled base. Saved versions are untouched, and runs, serve and
// the CLI never read the edit. Problems are as for SaveRequest; autosave
// should leave o.DropComments false so a commented file is refused
// (FLW-WRITE-001) instead of silently losing its comments.
func (w *Workspace) StoreEdit(t RequestTarget, base string, v model.RequestVersion, o SaveOptions) error {
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	v.Label = ""
	r.Edit, r.EditBase = &v, base
	return w.SaveRequest(t, r, o)
}

// PromoteEdit turns the edit into a saved version and clears the slot in
// one write. An empty asLabel overwrites the base version (FLW-WRITE-004 if
// the base no longer exists); a label adds a new version (FLW-WRITE-004 if
// it is invalid or taken). No edit is FLW-WRITE-003.
func (w *Workspace) PromoteEdit(t RequestTarget, asLabel string, o SaveOptions) error {
	asLabel = version.Normalize(asLabel) // "Version 2" and "v2" are "2", whoever asks
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	if r.Edit == nil {
		return diagErr(diag.CodeWriteEditNone, map[string]string{"request": r.Name, "action": "save"})
	}
	next, err := withPromotedEdit(r, asLabel)
	if err != nil {
		return err
	}
	return w.SaveRequest(t, next, o)
}

// DiscardEdit clears the edit slot. Having none is not an error.
func (w *Workspace) DiscardEdit(t RequestTarget, o SaveOptions) error {
	_, r, err := w.requestFor(t)
	if err != nil || r.Edit == nil {
		return err
	}
	r.Edit, r.EditBase = nil, ""
	return w.SaveRequest(t, r, o)
}

// withPromotedEdit returns r with its edit moved into the versions; r's own
// slices are not modified.
func withPromotedEdit(r model.Request, asLabel string) (model.Request, error) {
	promoted := *r.Edit
	r.Versions = slices.Clone(r.Versions)
	label := asLabel
	if label == "" {
		label = r.EditBase
	}
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == label })
	switch {
	case asLabel == "" && i < 0:
		return r, editLabelError(label, "the version it started from no longer exists")
	case asLabel != "" && sameLabelTaken(r.Versions, label):
		return r, editLabelError(label, "it already exists (or names the same version as one that does)")
	case asLabel != "":
		if _, err := version.Parse(label); err != nil {
			return r, editLabelError(label, err.Error())
		}
		promoted.Label = label
		r.Versions = append(r.Versions, promoted)
	default:
		promoted.Label = label
		r.Versions[i] = promoted
	}
	r.Edit, r.EditBase = nil, ""
	return r, nil
}

// sameLabelTaken reports whether a version already has label, or one that
// the file format reads as the same ("1" and "1.0").
func sameLabelTaken(versions []model.RequestVersion, label string) bool {
	want, err := version.Parse(label)
	return slices.ContainsFunc(versions, func(v model.RequestVersion) bool {
		if v.Label == label {
			return true
		}
		have, perr := version.Parse(v.Label)
		return err == nil && perr == nil && have.Compare(want) == 0
	})
}

func editLabelError(label, detail string) error {
	return diagErr(diag.CodeWriteEditLabel, map[string]string{"label": label, "detail": detail})
}
