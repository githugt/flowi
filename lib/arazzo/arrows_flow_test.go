// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodeFlow_should_give_back_the_failure_arrows_it_encoded(t *testing.T) {
	o := datasetFlowOptions()
	fl := datasetFlow()
	fl.Nodes[1].AfterFailureOf = []string{"place"}
	fl.Nodes[1].Needs = []string{"place"}
	fl.Nodes[1].AfterOutcome = []string{"place:not-found"}

	got, _, err := EncodeFlow(fl, o)
	if err != nil {
		t.Fatal(err)
	}
	back, diags := DecodeFlow(fl.File, fl.Path, got, o)

	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if !strings.Contains(string(got), "x-flowi-after-failure-of") {
		t.Errorf("the failure arrow is not written:\n%s", got)
	}
	if !reflect.DeepEqual(back.Nodes[1].AfterOutcome, []string{"place:not-found"}) {
		t.Errorf("after_outcome came back as %v", back.Nodes[1].AfterOutcome)
	}
	if !reflect.DeepEqual(back.Nodes[1].AfterFailureOf, []string{"place"}) || !reflect.DeepEqual(back.Nodes[1].Needs, []string{"place"}) {
		t.Errorf("arrows came back as needs %v, after_failure_of %v", back.Nodes[1].Needs, back.Nodes[1].AfterFailureOf)
	}
}

func TestDecodeFlow_should_give_back_an_unlinked_step(t *testing.T) {
	o := datasetFlowOptions()
	fl := datasetFlow()
	fl.Nodes[1].Unlinked = true

	got, _, err := EncodeFlow(fl, o)
	if err != nil {
		t.Fatal(err)
	}
	back, diags := DecodeFlow(fl.File, fl.Path, got, o)

	if len(diags) > 0 || !back.Nodes[1].Unlinked {
		t.Fatalf("diags %v, unlinked %v:\n%s", diags, back.Nodes[1].Unlinked, got)
	}
}
