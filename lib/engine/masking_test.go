// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

func TestRun_secret_should_be_masked_everywhere(t *testing.T) {
	f := newFixture(t)
	failing := flowNode("leak", "fetch", "url", "http://127.0.0.1:1/x?k={{vars.pw}}")
	failing.OnError = "continue"
	fl := f.addFlow("leaky",
		setNode("pw", "vars.pw", "{{secret.password}}"),
		flowNode("say", "log", "value", "pw={{vars.pw}}"),
		flowNode("remember", "note", "text", "pw={{vars.pw}}"),
		failing,
		flowNode("keep", "save", "key", "pw", "value", "{{ {'pw': vars.pw} }}"),
		flowNode("out", "digest", "format", "markdown"),
		flowNode("file", "digest", "sink", "file", "file", "d.md"),
		flowNode("hook", "digest", "sink", "webhook", "url", "{{env.base_url}}/echo/digest"),
		setValues("dump", model.SetValue{Name: "dumped", Value: "{{ {'pw': vars.pw} }}"}))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
	var everything bytes.Buffer
	for _, ev := range events {
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		everything.Write(data)
	}
	for _, write := range []func(io.Writer, report.Run) error{report.WriteJSON, report.WriteJUnit, report.WriteMarkdown, report.WriteText} {
		if err := write(&everything, res.Report); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range res.Saved {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		everything.Write(data)
	}
	saved, _ := os.ReadFile(res.ReportFile)
	everything.Write(saved)
	f.echoMu.Lock()
	everything.WriteString(strings.Join(f.echoBodies, ""))
	f.echoMu.Unlock()

	if strings.Contains(everything.String(), secretV) {
		t.Fatalf("secret leaked somewhere in:\n%s", everything.String())
	}
	if !strings.Contains(everything.String(), "••••••") {
		t.Fatal("expected masked values")
	}
	if len(res.Saved) < 2 || filepath.Base(res.ReportFile) != res.RunID+".json" {
		t.Fatalf("saved %v report %s", res.Saved, res.ReportFile)
	}
}
