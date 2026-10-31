// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/dataset/readonly"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// scriptedDataset records the query it was given and answers with a canned result.
type scriptedDataset struct {
	got dataset.Query
	res dataset.Result
	err error
}

func (s *scriptedDataset) Run(_ context.Context, q dataset.Query) (dataset.Result, error) {
	s.got = q
	return s.res, s.err
}

func twoCustomers() dataset.Result {
	return dataset.Result{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{int64(1), "Ada"}, {int64(2), "Grace"}},
		Duration: 3 * time.Millisecond,
	}
}

func datasetVersion(d model.Dataset) model.RequestVersion {
	if d.Driver == "" {
		d.Driver = model.DriverPostgres
	}
	if d.Server == "" {
		d.Server = "postgres://app@db:5432/shop"
	}
	if d.Query == "" {
		d.Query = "SELECT id, name FROM customers"
	}
	return model.RequestVersion{Dataset: &d}
}

func sendDatasetRequest(t *testing.T, runner *scriptedDataset, v model.RequestVersion, tweak func(*SendInput)) (SendResult, []Event) {
	t.Helper()
	f := newFixture(t)
	f.dataset = runner
	r := f.addRequest("customers/list", v)
	in := f.input(r)
	if tweak != nil {
		tweak(&in)
	}
	return f.send(in)
}

func TestSend_Dataset_should_run_the_query_and_make_the_rows_readable(t *testing.T) {
	runner := &scriptedDataset{res: twoCustomers()}
	v := datasetVersion(model.Dataset{
		Server: "postgres://app@{{env_host}}:5432/shop", Query: "SELECT id, name FROM customers WHERE id > $1", Params: []string{"{{id}}"}, Limit: 10,
	})
	v.Variables = []model.KeyValue{{Key: "env_host", Value: "db"}}
	v.Assert = []string{`row_count == 2`, `body.rows[0].name == "Ada"`, `rows[1].id == 2`, `columns[1] == "name"`, `outcome == "ok"`, `truncated == false`}
	v.Extract = []model.Extraction{{Name: "first", From: "body", Path: "$.rows[0].name"}}

	res, _ := sendDatasetRequest(t, runner, v, func(in *SendInput) { in.With = map[string]string{"id": "0"} })

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	q := runner.got
	if q.Driver != "postgres" || q.Server != "postgres://app@db:5432/shop" || q.Statement != "SELECT id, name FROM customers WHERE id > $1" || len(q.Params) != 1 || q.Params[0] != "0" || q.Limit != 10 {
		t.Fatalf("query %+v", q)
	}
	rec := res.Record
	if rec.Kind != KindDataset || rec.Outcome != OutcomeOK || rec.Request.Method != "SQL" || rec.Proto != "postgres" || rec.Status != 0 {
		t.Fatalf("record kind %q outcome %q method %q proto %q status %d", rec.Kind, rec.Outcome, rec.Request.Method, rec.Proto, rec.Status)
	}
	if got := res.Record.Outputs["first"]; got != "Ada" {
		t.Fatalf("outputs %v", res.Record.Outputs)
	}
}

func TestSend_Dataset_should_record_the_rows_as_one_json_body(t *testing.T) {
	runner := &scriptedDataset{res: twoCustomers()}

	res, _ := sendDatasetRequest(t, runner, datasetVersion(model.Dataset{}), nil)

	var body struct {
		Columns  []string         `json:"columns"`
		Rows     []map[string]any `json:"rows"`
		RowCount int              `json:"row_count"`
	}
	if err := json.Unmarshal([]byte(res.Record.Body.Text), &body); err != nil {
		t.Fatalf("body %q: %v", res.Record.Body.Text, err)
	}
	if body.RowCount != 2 || len(body.Rows) != 2 || body.Rows[1]["name"] != "Grace" || body.Columns[0] != "id" {
		t.Fatalf("body %+v", body)
	}
}

func TestSend_Dataset_should_say_when_rows_were_cut(t *testing.T) {
	res0 := twoCustomers()
	res0.Truncated = true
	v := datasetVersion(model.Dataset{})
	v.Assert = []string{`truncated == true`}

	res, _ := sendDatasetRequest(t, &scriptedDataset{res: res0}, v, nil)

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Dataset_should_not_leak_secrets(t *testing.T) {
	runner := &scriptedDataset{res: twoCustomers()}
	v := datasetVersion(model.Dataset{
		Server: "postgres://app:{{secret.password}}@db:5432/shop", Query: "SELECT * FROM t WHERE token = $1", Params: []string{"{{secret.password}}"},
	})

	res, _ := sendDatasetRequest(t, runner, v, nil)

	if !strings.Contains(runner.got.Server, secretV) || runner.got.Params[0] != secretV {
		t.Fatalf("the query must carry the real secret, got %+v", runner.got)
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Fatalf("secret in result: %s", data)
	}
}

func TestSend_Dataset_should_not_record_a_password_written_in_the_server(t *testing.T) {
	v := datasetVersion(model.Dataset{Server: "mysql://app:plain-pw@db:3306/shop", Driver: model.DriverMySQL})

	res, _ := sendDatasetRequest(t, &scriptedDataset{res: twoCustomers()}, v, nil)

	data, _ := json.Marshal(res)
	if strings.Contains(string(data), "plain-pw") {
		t.Fatalf("password in result: %s", data)
	}
}

func TestSend_Dataset_should_turn_failures_into_diagnostics(t *testing.T) {
	tests := map[string]struct {
		err  error
		want diag.Code
	}{
		"refused":  {readonly.ErrNotReadOnly, diag.CodeDatasetRequest},
		"driver":   {dataset.ErrUnknownDriver, diag.CodeDatasetRequest},
		"timeout":  {dataset.ErrTimeout, diag.CodeDatasetTimeout},
		"database": {errors.New("relation \"customers\" does not exist"), diag.CodeDatasetQuery},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res, _ := sendDatasetRequest(t, &scriptedDataset{err: tt.err}, datasetVersion(model.Dataset{}), nil)

			if res.Status != StatusError || !hasCode(res.Diags, tt.want) {
				t.Fatalf("status %s diags %v, want %s", res.Status, res.Diags, tt.want)
			}
		})
	}
}

func TestSend_Dataset_should_report_the_request_and_the_outcome_as_events(t *testing.T) {
	_, events := sendDatasetRequest(t, &scriptedDataset{res: twoCustomers()}, datasetVersion(model.Dataset{}), nil)

	var sent, received bool
	for _, ev := range events {
		switch e := ev.(type) {
		case RequestSent:
			sent = e.Method == "SQL"
		case ResponseReceived:
			received = e.Outcome == OutcomeOK
		}
	}
	if !sent || !received {
		t.Fatalf("sent %v received %v", sent, received)
	}
}

func TestPreviewSend_Dataset_should_mask_secrets_and_show_the_params(t *testing.T) {
	f := newFixture(t)
	v := datasetVersion(model.Dataset{
		Server: "postgres://app:{{secret.password}}@db:5432/shop", Query: "SELECT * FROM t WHERE a = $1 AND b = $2", Params: []string{"plain", "{{secret.password}}"},
	})
	r := f.addRequest("customers/list", v)

	p := PreviewSend(f.deps(), f.input(r), true)

	if p.Real == nil || !strings.Contains(p.Real.URL, secretV) {
		t.Fatalf("real %+v", p.Real)
	}
	masked, _ := json.Marshal(p.Masked)
	if strings.Contains(string(masked), secretV) || !strings.Contains(p.Masked.Body, "$1 = plain") || strings.Contains(p.Masked.Body, "$2 = "+secretV) {
		t.Fatalf("masked %s", masked)
	}
}

func TestSend_Dataset_should_let_pre_hooks_read_the_query(t *testing.T) {
	f := newFixture(t)
	f.dataset = &scriptedDataset{res: twoCustomers()}
	r := f.addRequest("customers/list", datasetVersion(model.Dataset{}))
	r.Hooks = model.Hooks{Pre: []model.HookStep{inlineHook(assertNode("a", `request.method == "SQL" && request.body == "SELECT id, name FROM customers"`))}}
	f.update(func(c *model.Collection) { c.Requests[0] = r })

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestRecord_Rows_should_read_the_count_and_the_cut_from_a_dataset_record(t *testing.T) {
	res0 := twoCustomers()
	res0.Truncated = true
	res, _ := sendDatasetRequest(t, &scriptedDataset{res: res0}, datasetVersion(model.Dataset{}), nil)

	count, truncated := res.Record.Rows()

	if count != 2 || !truncated {
		t.Fatalf("count %d truncated %v", count, truncated)
	}
}

func TestRecord_Rows_should_read_a_record_that_was_saved_and_read_back(t *testing.T) {
	var rec Record
	if err := json.Unmarshal([]byte(`{"kind":"dataset","body":{"json":{"row_count":7,"truncated":false}}}`), &rec); err != nil {
		t.Fatal(err)
	}

	count, truncated := rec.Rows()

	if count != 7 || truncated {
		t.Fatalf("count %d truncated %v", count, truncated)
	}
}

func TestRecord_Rows_should_be_zero_for_any_other_record(t *testing.T) {
	count, truncated := (&Record{Kind: KindGRPC}).Rows()

	if count != 0 || truncated {
		t.Fatalf("count %d truncated %v", count, truncated)
	}
}
