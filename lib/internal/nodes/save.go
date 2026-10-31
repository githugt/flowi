// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/model"
)

func saveSpec() Spec {
	return Spec{
		Type: "save", Group: "Output", Summary: "Save a value (masked) under the workspace's `saved/<flow-id>/`: overwrite, latest by status, or append.",
		Params: []Param{
			{Name: "key", Kind: ParamName, Required: true, Doc: "What the value is saved as: a plain file name without extension."},
			{Name: "value", Kind: ParamAny, Required: true, Doc: `What to save, e.g. "{{nodes.login}}".`},
			{Name: "mode", Kind: ParamEnum, Enum: []string{"overwrite", "latest-by-status", "append"}, Doc: "overwrite (default): `<key>.json`; latest-by-status: `<key>/<status>.json` from `value.status`; append: a line in `<key>.ndjson`."},
		},
		Run: runSave,
	}
}

func runSave(_ context.Context, n model.Node, s *State) Outcome {
	raw, _ := param(n, "value")
	v, err := eval(raw, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "value", err)}
	}
	name := stringParam(n, "key")
	if !names.Valid(name) {
		return failed(n, diag.CodeRunFileName, map[string]string{"name": name})
	}
	switch stringParam(n, "mode") {
	case "latest-by-status":
		status, ok := statusOf(v)
		if !ok {
			return failed(n, diag.CodeRunSaveStatus, nil)
		}
		return saveJSON(n, s, name+"/"+status+".json", v, false)
	case "append":
		return saveJSON(n, s, name+".ndjson", v, true)
	}
	return saveJSON(n, s, name+".json", v, false)
}

// saveJSON writes v masked: indented, or as one line when appending.
func saveJSON(n model.Node, s *State, file string, v any, appendTo bool) Outcome {
	var data []byte
	var err error
	if appendTo {
		data, err = json.Marshal(v)
	} else {
		data, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		return Outcome{Status: StatusError, Diags: diag.List{newDiag(n, diag.CodeNodeParam, map[string]string{"field": "value", "detail": err.Error()})}}
	}
	masked := s.Masker.String(string(data)) + "\n"
	return writeOutput(n, s, "saved", file, []byte(masked), appendTo)
}

// writeOutput writes through the runtime: a refused name is a flow error
// (FLW-RUN-017), a failed write a system error.
func writeOutput(n model.Node, s *State, dir, name string, data []byte, appendTo bool) Outcome {
	path, err := s.Runtime.WriteFile(dir, name, data, appendTo)
	switch {
	case errors.Is(err, ErrFileName):
		return failed(n, diag.CodeRunFileName, map[string]string{"name": name})
	case err != nil:
		return Outcome{Status: StatusError, Diags: diag.List{newDiag(n, diag.CodeRunWriteFailed, map[string]string{"name": name, "detail": err.Error()})}}
	}
	return Outcome{Outputs: map[string]any{"file": path}}
}

func statusOf(v any) (string, bool) {
	m, _ := v.(map[string]any)
	switch st := m["status"].(type) {
	case int:
		return strconv.Itoa(st), true
	case int64:
		return strconv.FormatInt(st, 10), true
	case float64:
		return strconv.Itoa(int(st)), st == float64(int(st))
	}
	return "", false
}
