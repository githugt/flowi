// SPDX-License-Identifier: Apache-2.0

// Package specoverlay applies OpenAPI Initiative Overlay documents (1.0 and 1.1:
// ordered actions, each a JSONPath target with update, remove or copy) to an
// OpenAPI or AsyncAPI description before it is imported. Overlays are how a
// description is fixed without editing it: add the operationIds it lacks,
// rename tags, drop internal operations. The original document stays as it was.
package specoverlay

import (
	"fmt"
	"path"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/overlay"

	"github.com/githugt/flowi-lib/diag"
)

// Apply applies each overlay in order to spec and returns the result. An
// overlay that is not valid, or whose action fails, is FLW-IMPORT-002 and
// nothing is applied; an action whose target matches nothing is a warning
// (FLW-IMPORT-001), the usual sign of a stale overlay.
func Apply(spec []byte, overlays [][]byte) ([]byte, diag.List) {
	return ApplyTo(spec, "", overlays)
}

// ApplyTo is Apply for a document known by name (a file name or URL). An
// overlay whose "extends" names a different document is applied all the same
// but warned about: "extends" says which document the overlay was written
// for, and one written for another is usually a mistake. An overlay with no
// "extends", or a document with no name, is not checked.
func ApplyTo(spec []byte, name string, overlays [][]byte) ([]byte, diag.List) {
	var warnings diag.List
	for i, raw := range overlays {
		ov, err := libopenapi.NewOverlayDocument(raw)
		if err != nil {
			return nil, invalid(i, err)
		}
		if w, ok := extendsOther(i, ov.Extends, name); ok {
			warnings = append(warnings, w)
		}
		res, err := overlay.Apply(spec, ov)
		if err != nil {
			return nil, invalid(i, err)
		}
		spec = res.Bytes
		for _, w := range res.Warnings {
			warnings = append(warnings, diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
				"what": fmt.Sprintf("overlay %d, target %s", i+1, w.Target), "how": "not applied", "detail": w.Message}))
		}
	}
	return spec, warnings
}

// extendsOther warns when extends names another document than name. Only the
// last path segment counts (an overlay written against a URL still fits the
// same file saved locally); a JSON Pointer or query after it is ignored.
func extendsOther(i int, extends, name string) (diag.Diagnostic, bool) {
	if extends == "" || name == "" || lastSegment(extends) == lastSegment(name) {
		return diag.Diagnostic{}, false
	}
	return diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
		"what": fmt.Sprintf("overlay %d extends %s", i+1, extends), "how": "applied anyway",
		"detail": fmt.Sprintf("the overlay says it was written for %s, not %s", lastSegment(extends), lastSegment(name))}), true
}

func lastSegment(ref string) string {
	ref, _, _ = strings.Cut(ref, "#")
	ref, _, _ = strings.Cut(ref, "?")
	return path.Base(strings.ReplaceAll(strings.TrimRight(ref, "/"), "\\", "/"))
}

func invalid(i int, err error) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": fmt.Sprintf("overlay %d: %v", i+1, err)})}
}
