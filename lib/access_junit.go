// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/xml"
	"fmt"
	"strconv"
)

type accessSuite struct {
	XMLName  xml.Name     `xml:"testsuite"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Cases    []accessCase `xml:"testcase"`
}

type accessCase struct {
	Name      string         `xml:"name,attr"`
	Classname string         `xml:"classname,attr"`
	Failure   *accessProblem `xml:"failure,omitempty"`
	Skipped   *accessProblem `xml:"skipped,omitempty"`
}

type accessProblem struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr,omitempty"`
	Text    string `xml:",chardata"`
}

// JUnit renders the report as a JUnit XML test suite for CI: one test case for each step and other identity.
// A case the other identity reached fails; one that got neither a refusal nor a success (a 400, no answer) is
// skipped, because it proves nothing either way.
func (r AccessReport) JUnit() ([]byte, error) {
	suite := accessSuite{Name: "access " + r.Flow}
	for _, st := range r.Steps {
		for _, a := range st.Attempts {
			c := accessCase{Name: fmt.Sprintf("%s as %s", st.ID, a.Identity), Classname: "access." + r.Flow}
			switch a.Verdict {
			case AccessAllowed:
				msg := fmt.Sprintf("%s reached %s: status %d (the owner %s got %d)", a.Identity, st.ID, a.Status, r.Owner, st.Owner)
				c.Failure = &accessProblem{Message: msg, Type: "access", Text: msg}
				suite.Failures++
			case AccessUnclear:
				c.Skipped = &accessProblem{Message: "status " + strconv.Itoa(a.Status) + " is neither a refusal nor a success"}
				suite.Skipped++
			}
			suite.Cases = append(suite.Cases, c)
		}
	}
	suite.Tests = len(suite.Cases)
	out, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("writing the access report as JUnit: %w", err)
	}
	return append([]byte(xml.Header), append(out, '\n')...), nil
}
