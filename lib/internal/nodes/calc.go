// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"fmt"
	"math"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func calcSpec() Spec {
	return Spec{
		Type: "calc", Group: "Data", MapForm: true,
		Summary: "Arithmetic on variables (+ - * / %, min, max) into variables.",
		Run:     runCalc,
	}
}

// runCalc renders each "<var>: template" and requires a finite number.
func runCalc(_ context.Context, n model.Node, s *State) Outcome {
	values, diags := map[string]any{}, diag.List(nil)
	for _, f := range n.Params {
		v, err := eval(f.Value, s)
		if err == nil {
			err = finiteNumber(v)
		}
		if err != nil {
			diags = append(diags, newDiag(n, diag.CodeRunCalcNumber, map[string]string{"name": f.Key, "detail": calcDetail(err)}))
			continue
		}
		values[f.Key] = v
	}
	return writeVars(s, values, diags)
}

func finiteNumber(v any) error {
	var f float64
	switch x := v.(type) {
	case int:
		return nil
	case int64:
		return nil
	case float64:
		f = x
	default:
		return fmt.Errorf("got %s", describe(v))
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("got %v (division by zero?)", f)
	}
	return nil
}

// calcDetail keeps template errors short; they already name the problem.
func calcDetail(err error) string { return err.Error() }

func describe(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%T %v", v, v)
}
