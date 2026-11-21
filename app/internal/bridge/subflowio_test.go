// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestSubflowIO_should_list_the_inputs_and_outputs_of_the_called_flow(t *testing.T) {
	// Before: the called flow's inputs and outputs were not shown on the step, so "with" was free text.
	f := model.Flow{
		Inputs:  []model.Input{{Name: "user", Type: "string"}, {Name: "page", Type: "int", Default: "1"}},
		Outputs: []model.Output{{Name: "token", Type: "string"}},
	}
	f.ID = "F1"
	c := model.Collection{Flows: []model.Flow{f}}

	ins, outs := subflowIO(c, "F1")
	noIns, noOuts := subflowIO(c, "nope")

	if len(ins) != 2 || !ins[0].Required || ins[1].Required || ins[1].Default != "1" {
		t.Fatalf("inputs %+v", ins)
	}
	if len(outs) != 1 || outs[0].Name != "token" || len(noIns) != 0 || len(noOuts) != 0 {
		t.Fatalf("outputs %+v, unknown flow %+v %+v", outs, noIns, noOuts)
	}
}
