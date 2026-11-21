// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// prober is a transport with a connection test: it records what it was asked to test and answers with err.
type prober struct {
	scripted
	probed transport.Operation
	err    error
	calls  int
}

func (p *prober) Probe(_ context.Context, op transport.Operation) error {
	p.calls++
	p.probed = op
	return p.err
}

func probeInput(t *testing.T, f *fixture, v model.RequestVersion) SendInput {
	t.Helper()
	return f.input(f.addRequest("orders/placed", v))
}

func TestProbeSend_Given_a_message_request_Then_it_connects_with_the_credentials_and_sends_nothing(t *testing.T) {
	// Before: a wrong broker address or password showed only after a full send or a 30 s timeout.
	p := &prober{}
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": p}
	v := asyncVersion(model.Async{
		Channel: "orders", Send: &model.AsyncSend{Body: model.Body{JSON: `{"x":1}`}}, Receive: &model.AsyncReceive{Count: 3},
		Binding: []model.KeyValue{{Key: "username", Value: "u"}, {Key: "password", Value: "{{secret.password}}"}},
	})

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, v))

	if got.Status != ProbeOK || p.calls != 1 || got.Detail == "" {
		t.Fatalf("probe %+v calls %d", got, p.calls)
	}
	if p.probed.Send != nil || p.probed.Receive != nil || p.scripted.got.Send != nil || p.probed.Binding["password"] != secretV {
		t.Fatalf("the test must carry the real password and no message: %+v", p.probed)
	}
	if strings.Contains(got.Target, secretV) {
		t.Fatalf("target %q", got.Target)
	}
}

func TestProbeSend_Given_a_refusal_Then_the_service_words_are_shown_with_secrets_masked(t *testing.T) {
	p := &prober{err: errors.New("connecting to mqtt://broker: not authorized for " + secretV)}
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": p}
	v := asyncVersion(model.Async{Binding: []model.KeyValue{{Key: "password", Value: "{{secret.password}}"}}})

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, v))

	if got.Status != ProbeFailed || !strings.Contains(got.Detail, "not authorized") || strings.Contains(got.Detail, secretV) {
		t.Fatalf("probe %+v", got)
	}
}

func TestProbeSend_Given_a_transport_without_a_test_Then_it_says_so_and_nothing_runs(t *testing.T) {
	tr := &scripted{}
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": tr}

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, asyncVersion(model.Async{})))

	if got.Status != ProbeUnsupported || tr.got.Server != "" {
		t.Fatalf("probe %+v, transport saw %+v", got, tr.got)
	}
}

func TestProbeSend_Given_an_unresolved_variable_Then_the_diagnostics_come_back_and_nothing_connects(t *testing.T) {
	p := &prober{}
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": p}
	v := asyncVersion(model.Async{Server: "mqtt://{{env.no_such_host}}:1883"})

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, v))

	if got.Status != ProbeFailed || !got.Diags.HasErrors() || p.calls != 0 {
		t.Fatalf("probe %+v calls %d", got, p.calls)
	}
	if got.Diags[0].Code != diag.CodeTemplateUndefined {
		t.Logf("code %s", got.Diags[0].Code)
	}
}

func TestProbeSend_Given_a_database_Then_it_runs_select_1_and_reports_the_error(t *testing.T) {
	// Before: a wrong DSN showed only after the query ran, or after a 30 s timeout.
	runner := &scriptedDataset{res: twoCustomers()}
	f := newFixture(t)
	f.dataset = runner
	in := f.input(f.addRequest("customers/list", datasetVersion(model.Dataset{Query: "DELETE FROM customers", Params: []string{"1"}, Writes: true})))

	ok := ProbeSend(t.Context(), f.deps(), in)
	runner.err = errors.New(`password authentication failed for user "app"`)
	bad := ProbeSend(t.Context(), f.deps(), in)

	if ok.Status != ProbeOK || runner.got.Statement != "SELECT 1" || len(runner.got.Params) != 0 || runner.got.AllowWrites {
		t.Fatalf("ok %+v query %+v", ok, runner.got)
	}
	if bad.Status != ProbeFailed || !strings.Contains(bad.Detail, "password authentication failed") {
		t.Fatalf("bad %+v", bad)
	}
}

func TestProbeSend_Given_a_data_file_or_an_http_request_Then_it_is_unsupported(t *testing.T) {
	f := newFixture(t)
	file := model.RequestVersion{File: &model.DataFile{Format: model.FormatCSV, Path: "a.csv", Query: "SELECT * FROM data"}}

	fileProbe := ProbeSend(t.Context(), f.deps(), probeInput(t, f, file))
	httpProbe := ProbeSend(t.Context(), f.deps(), f.input(f.addRequest("h", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/x"})))

	if fileProbe.Status != ProbeUnsupported || httpProbe.Status != ProbeUnsupported {
		t.Fatalf("file %+v http %+v", fileProbe, httpProbe)
	}
}
