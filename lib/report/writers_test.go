// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/report"
)

func TestWriteJSON_should_match_golden(t *testing.T) {
	var buf bytes.Buffer

	err := report.WriteJSON(&buf, fixtureRun())

	if err != nil {
		t.Fatal(err)
	}
	golden(t, "run.golden.json", buf.Bytes())
}

func TestWriteMarkdown_should_match_golden(t *testing.T) {
	var buf bytes.Buffer

	err := report.WriteMarkdown(&buf, fixtureRun())

	if err != nil {
		t.Fatal(err)
	}
	golden(t, "run.golden.md", buf.Bytes())
}

func TestWriteText_should_match_golden(t *testing.T) {
	var buf bytes.Buffer

	err := report.WriteText(&buf, fixtureRun())

	if err != nil {
		t.Fatal(err)
	}
	golden(t, "run.golden.txt", buf.Bytes())
}

func TestWriteJUnit_should_validate_against_the_jenkins_schema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint not installed; install libxml2-utils to run the JUnit schema check")
	}
	out := filepath.Join(t.TempDir(), "junit.xml")
	var buf bytes.Buffer
	if err := report.WriteJUnit(&buf, fixtureRun()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := exec.CommandContext(t.Context(), xmllint, "--noout", "--schema", filepath.Join("testdata", "junit-10.xsd"), out).CombinedOutput()

	if err != nil {
		t.Fatalf("xmllint: %v\n%s\n%s", err, res, buf.Bytes())
	}
}

type junitSuites struct {
	Suites []struct {
		Tests    int `xml:"tests,attr"`
		Failures int `xml:"failures,attr"`
		Errors   int `xml:"errors,attr"`
		Skipped  int `xml:"skipped,attr"`
		Props    []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"properties>property"`
		Cases []struct {
			Name      string    `xml:"name,attr"`
			Classname string    `xml:"classname,attr"`
			Failure   *struct{} `xml:"failure"`
			Error     *struct{} `xml:"error"`
			Skipped   *struct{} `xml:"skipped"`
			SystemErr string    `xml:"system-err"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

func TestWriteJUnit_should_map_statuses(t *testing.T) {
	r := report.Run{ID: "R1", FlowName: "F", FlowPath: "dir/f", Collection: "c", Env: "local", Status: "failed", Seed: 7,
		Nodes: []report.Node{
			{Path: "ok", Status: "passed"},
			{Path: "bad", Status: "failed", Diagnostics: []report.Diagnostic{{Code: "FLW-RUN-001", Message: "assertion failed: x"}}},
			{Path: "down", Status: "error", Diagnostics: []report.Diagnostic{{Code: "FLW-HTTP-001", Message: "request failed"}}},
			{Path: "off", Status: "skipped"},
			{Path: "stopped", Status: "cancelled"},
			{Path: "handled", Status: "failed", Handled: true, Diagnostics: []report.Diagnostic{{Code: "FLW-RUN-001", Message: "assertion failed: y"}}},
		}}
	var buf bytes.Buffer

	err := report.WriteJUnit(&buf, r)

	if err != nil {
		t.Fatal(err)
	}
	var got junitSuites
	if err := xml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	s := got.Suites[0]
	if s.Tests != 6 || s.Failures != 1 || s.Errors != 1 || s.Skipped != 2 {
		t.Fatalf("counts %+v", s)
	}
	h := s.Cases[5]
	if h.Failure != nil || h.Error != nil || !strings.Contains(h.SystemErr, "assertion failed: y") {
		t.Fatalf("handled case %+v", h)
	}
	if s.Cases[0].Classname != "c.dir.f" || s.Cases[1].Failure == nil || s.Cases[2].Error == nil || s.Cases[3].Skipped == nil || s.Cases[4].Skipped == nil {
		t.Fatalf("cases %+v", s.Cases)
	}
	props := map[string]string{}
	for _, p := range s.Props {
		props[p.Name] = p.Value
	}
	if props["run_id"] != "R1" || props["seed"] != "7" || props["env"] != "local" || props["status"] != "failed" {
		t.Fatalf("properties %v", props)
	}
}

func TestWriteJUnit_should_escape_markup_in_messages(t *testing.T) {
	msg := `a < b & "c" ]]> d`
	r := report.Run{FlowName: "F", Nodes: []report.Node{{Path: "x", Status: "failed",
		Diagnostics: []report.Diagnostic{{Code: "FLW-RUN-001", Message: msg}}}}}
	var buf bytes.Buffer

	err := report.WriteJUnit(&buf, r)

	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Suite struct {
			Case struct {
				Failure struct {
					Message string `xml:"message,attr"`
					Text    string `xml:",chardata"`
				} `xml:"failure"`
			} `xml:"testcase"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, buf.Bytes())
	}
	if f := got.Suite.Case.Failure; f.Message != msg || !strings.Contains(f.Text, msg) {
		t.Fatalf("failure %+v", f)
	}
}

func TestRun_Leaves_and_Find(t *testing.T) {
	r := fixtureRun()

	leaves := r.Leaves()
	node, found := r.Find("users[1]/create")
	hookNode, hookFound := r.Find("login/pre:collection:0/sign")
	_, missing := r.Find("nope")

	var paths []string
	for _, l := range leaves {
		paths = append(paths, l.Path)
	}
	want := "login flaky users[0]/create users[1]/create both/b1/verify both/b2/note"
	if strings.Join(paths, " ") != want {
		t.Fatalf("leaves %v", paths)
	}
	if !found || node.Duration != 90*time.Millisecond || !hookFound || hookNode.ID != "sign" || missing {
		t.Fatalf("find: %v %v %v", found, hookFound, missing)
	}
}

func TestWriters_Given_running_node_and_unlabelled_lane_Then_no_question_marks_or_empty_items(t *testing.T) {
	r := report.Run{FlowName: "F", Status: "running", Nodes: []report.Node{
		{Path: "sub", ID: "sub", Type: "subflow", Status: "passed", Lanes: []report.Lane{{Label: "", Nodes: []report.Node{{Path: "sub/a", ID: "a", Type: "note", Status: "passed"}}}}},
		{Path: "d", ID: "d", Type: "digest", Status: "running"},
	}}
	for name, write := range map[string]func(io.Writer, report.Run) error{"markdown": report.WriteMarkdown, "text": report.WriteText} {
		var buf bytes.Buffer

		if err := write(&buf, r); err != nil {
			t.Fatal(err)
		}

		out := buf.String()
		if strings.Contains(out, "?") || strings.Contains(out, "  - \n") || strings.Contains(out, "\n    \n") || !strings.Contains(out, "⋯") {
			t.Errorf("%s:\n%s", name, out)
		}
	}
}
