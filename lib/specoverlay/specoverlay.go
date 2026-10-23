// SPDX-License-Identifier: Apache-2.0

// Package specoverlay applies OpenAPI Initiative Overlay documents (1.0 and 1.1:
// ordered actions, each a JSONPath target with update, remove or copy) to an
// OpenAPI or AsyncAPI description before it is imported. Overlays are how a
// description is fixed without editing it: add the operationIds it lacks,
// rename tags, drop internal operations. The original document stays as it was.
package specoverlay

import (
	"fmt"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/overlay"

	"github.com/githugt/flowi-lib/diag"
)

// Apply applies each overlay in order to spec and returns the result. An
// overlay that is not valid, or whose action fails, is FLW-IMPORT-002 and
// nothing is applied; an action whose target matches nothing is a warning
// (FLW-IMPORT-001), the usual sign of a stale overlay.
func Apply(spec []byte, overlays [][]byte) ([]byte, diag.List) {
	var warnings diag.List
	for i, raw := range overlays {
		ov, err := libopenapi.NewOverlayDocument(raw)
		if err != nil {
			return nil, invalid(i, err)
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

func invalid(i int, err error) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": fmt.Sprintf("overlay %d: %v", i+1, err)})}
}
