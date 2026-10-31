// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"errors"

	"github.com/githugt/flowi/lib/internal/yamldoc"
)

// OperationRef is the operation an imported request version came from, as
// the document writes it: what a spec overlay needs to point at it.
type OperationRef struct {
	RequestPath string // the request Import makes of it
	Label       string // the version label
	Method      string // lower case, as in the document
	Path        string // the path template, as in the document
	OperationID string
	Op          map[string]any // the operation object, references not followed
	PathParams  []any          // the path item's parameters, shared by its operations
}

// ErrSwagger2 is returned by OperationsOf for a Swagger 2.0 document: Import
// converts it first, so an overlay could not point into it.
var ErrSwagger2 = errors.New("a Swagger 2.0 document cannot take overlays from Flowi; convert it to OpenAPI 3 first")

// OperationsOf lists the operations of an OpenAPI 3 document with the
// request path and version label Import gives each.
func OperationsOf(data []byte) ([]OperationRef, error) {
	if isSwagger2(data) {
		return nil, ErrSwagger2
	}
	root, err := yamldoc.Decode(data)
	if err != nil {
		return nil, err
	}
	groups := groupOperations(operations(root))
	paths := requestPaths(groups)
	var out []OperationRef
	for i, g := range groups {
		for _, op := range g {
			id, _ := op.op["operationId"].(string)
			out = append(out, OperationRef{RequestPath: paths[i], Label: op.label, Method: op.method, Path: op.path,
				OperationID: id, Op: op.op, PathParams: op.pathParams})
		}
	}
	return out, nil
}
