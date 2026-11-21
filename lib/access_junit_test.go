// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/xml"
	"strings"
	"testing"
)

// Before: the access check printed a table only, so a CI job could not show which step and which user broke the rule.
func TestAccessReport_JUnit_should_fail_the_case_another_user_reached_and_skip_one_that_proves_nothing(t *testing.T) {
	rep := AccessReport{Flow: "notes", Owner: "alice", Steps: []AccessStep{{
		ID: "read", Owner: 200, Attempts: []AccessAttempt{
			{Identity: "bob", Status: 403, Verdict: AccessDenied},
			{Identity: "carol", Status: 200, Verdict: AccessAllowed},
			{Identity: "dave", Status: 500, Verdict: AccessUnclear},
		},
	}}}

	out, err := rep.JUnit()

	var suite struct {
		Tests    int `xml:"tests,attr"`
		Failures int `xml:"failures,attr"`
		Skipped  int `xml:"skipped,attr"`
		Cases    []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
			Skipped *struct{} `xml:"skipped"`
		} `xml:"testcase"`
	}
	if err != nil || xml.Unmarshal(out, &suite) != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if suite.Tests != 3 || suite.Failures != 1 || suite.Skipped != 1 || len(suite.Cases) != 3 {
		t.Fatalf("counts %+v\n%s", suite, out)
	}
	if suite.Cases[0].Name != "read as bob" || suite.Cases[0].Failure != nil || suite.Cases[0].Skipped != nil {
		t.Errorf("a refusal should pass: %+v", suite.Cases[0])
	}
	if f := suite.Cases[1].Failure; f == nil || !strings.Contains(f.Message, "carol reached read: status 200") {
		t.Errorf("a reached step should fail: %+v", suite.Cases[1])
	}
	if suite.Cases[2].Skipped == nil {
		t.Errorf("an unclear answer should be skipped: %+v", suite.Cases[2])
	}
}
