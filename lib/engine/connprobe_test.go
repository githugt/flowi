// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi/lib/dataset"
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
	if p.probed.Send != nil || p.probed.Receive != nil || p.got.Send != nil || p.probed.Binding["password"] != secretV {
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

// tabled is a dataset source that can also list the tables of a file, as the real file sources do.
type tabled struct {
	scriptedDataset
	tables []dataset.TableInfo
	tErr   error
	asked  dataset.Query
}

func (t *tabled) Tables(_ context.Context, q dataset.Query) ([]dataset.TableInfo, error) {
	t.asked = q
	return t.tables, t.tErr
}

func TestProbeSend_Given_a_data_file_Then_it_checks_the_file_exists_and_reads(t *testing.T) {
	// Before: a data file request had no test, so a wrong path showed only when the query ran.
	runner := &tabled{tables: []dataset.TableInfo{{Name: "people", Rows: 3}}}
	f := newFixture(t)
	f.dataset = runner
	file := model.RequestVersion{File: &model.DataFile{Format: model.FormatCSV, Path: "a.csv", Query: "SELECT * FROM people"}}
	in := probeInput(t, f, file)

	ok := ProbeSend(t.Context(), f.deps(), in)
	runner.tErr = errors.New(`the path "a.csv" cannot be read: no such file or directory`)
	missing := ProbeSend(t.Context(), f.deps(), in)

	if ok.Status != ProbeOK || !strings.Contains(ok.Detail, "reads as CSV") || !strings.Contains(ok.Detail, "people") || runner.got.Statement != "" {
		t.Fatalf("ok %+v: the file is read for its tables, no query runs (%+v)", ok, runner.got)
	}
	if missing.Status != ProbeFailed || !strings.Contains(missing.Detail, "cannot be read") {
		t.Fatalf("missing %+v", missing)
	}
}

func TestProbeSend_Given_a_data_file_source_that_cannot_list_tables_Then_it_is_unsupported(t *testing.T) {
	f := newFixture(t)
	f.dataset = &scriptedDataset{}
	file := model.RequestVersion{File: &model.DataFile{Format: model.FormatCSV, Path: "a.csv", Query: "SELECT * FROM data"}}

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, file))

	if got.Status != ProbeUnsupported {
		t.Fatalf("probe %+v", got)
	}
}

func TestProbeSend_Given_an_http_request_Then_it_sends_a_head_with_no_body_and_reads_the_status(t *testing.T) {
	// Before: an HTTP request had no connection test: a wrong host or a refused token showed only on a real send.
	var method, body, auth atomic.Value
	var code atomic.Int32
	code.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		method.Store(r.Method)
		body.Store(string(data))
		auth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(int(code.Load()))
	}))
	defer srv.Close()
	f := newFixture(t)
	in := f.input(f.addRequest("orders/create", model.RequestVersion{
		Method: "POST", URL: srv.URL + "/orders", Body: model.Body{JSON: `{"x":1}`},
		Headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer t"}},
	}))

	ok := ProbeSend(t.Context(), f.deps(), in)
	code.Store(401)
	refused := ProbeSend(t.Context(), f.deps(), in)
	code.Store(405)
	noHead := ProbeSend(t.Context(), f.deps(), in)
	code.Store(404)
	missing := ProbeSend(t.Context(), f.deps(), in)

	if method.Load() != "HEAD" || body.Load() != "" || auth.Load() != "Bearer t" {
		t.Fatalf("sent %v with body %q and authorization %q: a HEAD, no body, the same headers", method.Load(), body.Load(), auth.Load())
	}
	if ok.Status != ProbeOK || !strings.Contains(ok.Detail, "200") {
		t.Fatalf("ok %+v", ok)
	}
	if refused.Status != ProbeFailed || !strings.Contains(refused.Detail, "authentication") {
		t.Fatalf("refused %+v", refused)
	}
	if noHead.Status != ProbeOK || !strings.Contains(noHead.Detail, "does not answer HEAD") {
		t.Fatalf("a server that refuses HEAD is still reached: %+v", noHead)
	}
	if missing.Status != ProbeFailed || !strings.Contains(missing.Detail, "nothing at this address") {
		t.Fatalf("missing %+v", missing)
	}
}

func TestProbeSend_Given_an_http_server_that_is_not_there_Then_it_fails_with_the_reason(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	f := newFixture(t)

	got := ProbeSend(t.Context(), f.deps(), f.input(f.addRequest("h", model.RequestVersion{Method: "GET", URL: url + "/x"})))

	if got.Status != ProbeFailed || !got.Diags.HasErrors() {
		t.Fatalf("probe %+v", got)
	}
}

func TestProbeSend_Given_a_grpc_server_that_is_not_there_Then_it_fails_and_calls_no_method(t *testing.T) {
	// Before: a gRPC request had no connection test.
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	f := newFixture(t)
	v := model.RequestVersion{GRPC: &model.GRPC{Server: "grpc://" + addr, Method: "p.S/M"}}

	got := ProbeSend(t.Context(), f.deps(), probeInput(t, f, v))

	if got.Status != ProbeFailed || got.Detail == "" {
		t.Fatalf("probe %+v", got)
	}
}
