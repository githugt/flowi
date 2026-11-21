// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// JUnit XML as CI systems read it (the Jenkins junit-10.xsd): one suite per
// run, one test case per leaf node instance.
type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Errors     int             `xml:"errors,attr"`
	Skipped    int             `xml:"skipped,attr"`
	Time       string          `xml:"time,attr"`
	Timestamp  string          `xml:"timestamp,attr,omitempty"`
	ID         string          `xml:"id,attr,omitempty"`
	Properties []junitProperty `xml:"properties>property"`
	Cases      []junitCase     `xml:"testcase"`
}

type junitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitProblem `xml:"failure"`
	Error     *junitProblem `xml:"error"`
	Skipped   *junitProblem `xml:"skipped"`
	// Flaky is the Surefire way to say a test passed only after a retry (<flakyFailure>), which CI servers that read
	// JUnit report as flaky rather than failed.
	Flaky *junitProblem `xml:"flakyFailure"`
	// SystemOut lists what each check of a request said: a passed check is no test case of its own.
	SystemOut string `xml:"system-out,omitempty"`
	SystemErr string `xml:"system-err,omitempty"`
}

type junitProblem struct {
	Type    string `xml:"type,attr,omitempty"`
	Message string `xml:"message,attr,omitempty"`
	Text    string `xml:",chardata"`
}

// WriteJUnit writes r as JUnit XML. Statuses map as: failed → <failure>,
// error and aborted → <error>, skipped and cancelled → <skipped>. Handled
// failures pass, with their diagnostics in <system-err>.
func WriteJUnit(w io.Writer, r Run) error {
	suite := junitSuite{
		Name: r.FlowName, Time: seconds(r.Duration), ID: r.ID,
		Properties: []junitProperty{
			{Name: "run_id", Value: r.ID}, {Name: "flow", Value: r.FlowPath}, {Name: "seed", Value: strconv.FormatUint(r.Seed, 10)},
			{Name: "env", Value: r.Env}, {Name: "status", Value: r.Status},
		},
	}
	if !r.Started.IsZero() {
		suite.Timestamp = r.Started.UTC().Format(time.RFC3339)
	}
	classname := junitClassname(r)
	flaky, checksPassed, checksTotal := 0, 0, 0
	for _, n := range r.Leaves() {
		passed, total := n.ChecksPassed()
		checksPassed, checksTotal = checksPassed+passed, checksTotal+total
		c := junitCaseFor(n, classname)
		suite.count(c)
		suite.Cases = append(suite.Cases, c)
		if n.Flaky {
			flaky++
		}
	}
	if checksTotal > 0 {
		suite.Properties = append(suite.Properties, junitProperty{Name: "checks", Value: fmt.Sprintf("%d/%d", checksPassed, checksTotal)})
	}
	if flaky > 0 {
		suite.Properties = append(suite.Properties, junitProperty{Name: "flaky", Value: strconv.Itoa(flaky)})
	}
	doc := junitSuites{Name: r.FlowName, Tests: suite.Tests, Failures: suite.Failures, Errors: suite.Errors, Time: suite.Time, Suites: []junitSuite{suite}}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("writing JUnit report: %w", err)
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing JUnit report: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// junitClassname groups a flow's cases like a package: collection.dir.flow.
func junitClassname(r Run) string {
	parts := []string{}
	if r.Collection != "" {
		parts = append(parts, r.Collection)
	}
	if r.FlowPath != "" {
		parts = append(parts, strings.ReplaceAll(r.FlowPath, "/", "."))
	}
	return strings.Join(parts, ".")
}

func junitCaseFor(n Node, classname string) junitCase {
	c := junitCase{Name: n.Path, Classname: classname, Time: seconds(n.Duration)}
	problem := junitProblemFor(n)
	c.SystemOut = checksListing(n)
	if n.Flaky {
		c.Flaky = &junitProblem{Type: "flaky", Message: fmt.Sprintf("passed on try %d: it failed before", n.Tries)}
	}
	switch {
	case n.Handled:
		c.SystemErr = problem.Text
	case n.Status == "failed":
		c.Failure = &problem
	case n.Status == "error" || n.Status == "aborted":
		c.Error = &problem
	case n.Status == "skipped" || n.Status == "cancelled":
		c.Skipped = &junitProblem{Message: n.Status}
	}
	return c
}

// checksListing is "3/4 checks passed" and a line for each check, for the system-out of a case; "" without checks.
func checksListing(n Node) string {
	passed, total := n.ChecksPassed()
	if total == 0 {
		return ""
	}
	lines := []string{fmt.Sprintf("%d/%d checks passed", passed, total)}
	for _, c := range n.Checks {
		mark := map[string]string{"passed": "✓", "failed": "✗"}[c.Verdict]
		if mark == "" {
			mark = "!"
		}
		line := mark + " " + c.Line
		if c.Note != "" {
			line += " — " + c.Note
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// junitProblemFor names the first diagnostic and lists them all.
func junitProblemFor(n Node) junitProblem {
	if len(n.Diagnostics) == 0 {
		return junitProblem{Type: n.Status, Message: n.Status}
	}
	lines := make([]string, len(n.Diagnostics))
	for i, d := range n.Diagnostics {
		lines[i] = d.Code + " " + d.Message
	}
	first := n.Diagnostics[0]
	return junitProblem{Type: first.Code, Message: first.Message, Text: strings.Join(lines, "\n")}
}

func (s *junitSuite) count(c junitCase) {
	s.Tests++
	switch {
	case c.Failure != nil:
		s.Failures++
	case c.Error != nil:
		s.Errors++
	case c.Skipped != nil:
		s.Skipped++
	}
}
