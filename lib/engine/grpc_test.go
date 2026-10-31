// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/transport"
)

// scriptedGRPC is a gRPC runner that records the call it was given and
// answers with a canned result, calling OnMessage for the messages received
// like the real one does.
type scriptedGRPC struct {
	got  grpcx.Call
	res  grpcx.Result
	err  error
	hold bool // wait for the context to end
}

func (s *scriptedGRPC) Run(ctx context.Context, c grpcx.Call) (grpcx.Result, error) {
	s.got = c
	if s.hold {
		<-ctx.Done()
		partial := s.res
		partial.Code = codes.Canceled
		return partial, nil
	}
	if c.Receive != nil && c.Receive.OnMessage != nil {
		for _, m := range s.res.Messages {
			if m.Direction == "received" {
				c.Receive.OnMessage(m)
			}
		}
	}
	return s.res, s.err
}

// okOrder is a successful unary result: the request as sent, the response,
// response metadata and a trailer.
func okOrder() grpcx.Result {
	return grpcx.Result{
		Code: codes.OK,
		Messages: []transport.Message{
			{Direction: "sent", Body: []byte(`{"id":"7"}`)},
			{Direction: "received", Body: []byte(`{"id":"7","name":"order-7"}`)},
		},
		Header:  []transport.Header{{Name: "x-req", Value: "1"}},
		Trailer: []transport.Header{{Name: "x-trail", Value: "done"}},
	}
}

func grpcVersion(g model.GRPC) model.RequestVersion {
	if g.Server == "" {
		g.Server = "grpc://api:50051"
	}
	if g.Method == "" {
		g.Method = "acme.Orders/Get"
	}
	return model.RequestVersion{GRPC: &g}
}

func sendGRPC(t *testing.T, runner *scriptedGRPC, v model.RequestVersion, tweak func(*SendInput)) (SendResult, *fixture) {
	t.Helper()
	f := newFixture(t)
	f.grpc = runner
	r := f.addRequest("orders/get", v)
	in := f.input(r)
	if tweak != nil {
		tweak(&in)
	}
	res, _ := f.send(in)
	return res, f
}

func TestSend_GRPC_should_call_the_method_and_record_the_response(t *testing.T) {
	runner := &scriptedGRPC{res: okOrder()}
	v := grpcVersion(model.GRPC{Method: "acme.Orders/{{var.m}}", Body: model.Body{JSON: `{"id":"{{id}}"}`}, Proto: []string{"protos/o.proto"}, Import: []string{"protos"}})
	v.Headers = []model.KeyValue{{Key: "X-Token", Value: "abc"}}
	v.Variables = []model.KeyValue{{Key: "m", Value: "Get"}}
	v.Assert = []string{`code == "OK"`, `body.name == "order-7"`, `headers["x-req"] == "1"`, `trailers["x-trail"] == "done"`, `received == 1`, `outcome == "ok"`}

	res, f := sendGRPC(t, runner, v, func(in *SendInput) { in.With = map[string]string{"id": "7"} })

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	rec := res.Record
	if rec.Kind != KindGRPC || rec.Request.Method != "GRPC" || rec.Request.URL != "grpc://api:50051/acme.Orders/Get" || rec.Outcome != OutcomeOK || rec.GRPCCode != "OK" {
		t.Fatalf("record kind %q request %+v outcome %q code %q", rec.Kind, rec.Request, rec.Outcome, rec.GRPCCode)
	}
	if len(rec.Messages) != 2 || rec.Body.JSON == nil || len(rec.Trailers) != 1 || len(rec.Header) != 1 {
		t.Fatalf("messages %d body %+v trailers %v headers %v", len(rec.Messages), rec.Body, rec.Trailers, rec.Header)
	}
	got := runner.got
	if got.Server != "grpc://api:50051" || got.Method != "acme.Orders/Get" || string(got.JSON) != `{"id":"7"}` || got.Root != f.coll().Root ||
		len(got.Protos) != 1 || len(got.Imports) != 1 || len(got.Metadata) != 1 || got.Metadata[0].Value != "abc" {
		t.Fatalf("call %+v", got)
	}
}

func TestSend_GRPC_should_ask_for_gRPC_Web_when_the_version_does(t *testing.T) {
	runner := &scriptedGRPC{res: okOrder()}
	v := grpcVersion(model.GRPC{Server: "https://proxy.example/api", Proto: []string{"protos/o.proto"}, Web: true})

	res, _ := sendGRPC(t, runner, v, nil)

	if res.Status != StatusPassed || !runner.got.Web || runner.got.Server != "https://proxy.example/api" {
		t.Fatalf("status %s call %+v", res.Status, runner.got)
	}
}

func TestSend_GRPC_should_treat_a_non_ok_code_as_a_result_an_assertion_can_check(t *testing.T) {
	notFound := grpcx.Result{Code: codes.NotFound, Message: "no order 7", Messages: []transport.Message{{Direction: "sent", Body: []byte(`{"id":"7"}`)}}}
	v := grpcVersion(model.GRPC{})
	v.Assert = []string{`code == "NOT_FOUND"`}

	res, _ := sendGRPC(t, &scriptedGRPC{res: notFound}, v, nil)

	if res.Status != StatusPassed {
		t.Fatalf("a test that expects NOT_FOUND must pass: status %s diags %v", res.Status, res.Diags)
	}
	rec := res.Record
	if rec.Outcome != OutcomeError || rec.GRPCCode != "NOT_FOUND" || !strings.Contains(rec.Body.Text, "no order 7") {
		t.Fatalf("outcome %q code %q body %+v, want the status message shown as the body", rec.Outcome, rec.GRPCCode, rec.Body)
	}
}

func TestSend_GRPC_should_fail_the_default_assertion_on_a_non_ok_code(t *testing.T) {
	v := grpcVersion(model.GRPC{})
	v.Assert = []string{`code == "OK"`}

	res, _ := sendGRPC(t, &scriptedGRPC{res: grpcx.Result{Code: codes.Internal, Message: "boom"}}, v, nil)

	if res.Status != StatusFailed {
		t.Fatalf("status %s", res.Status)
	}
}

func TestSend_GRPC_should_record_a_missed_deadline_as_a_timeout(t *testing.T) {
	res, _ := sendGRPC(t, &scriptedGRPC{res: grpcx.Result{Code: codes.DeadlineExceeded, Message: "context deadline exceeded"}}, grpcVersion(model.GRPC{}), nil)

	if res.Record == nil || res.Record.Outcome != OutcomeTimeout || res.Record.GRPCCode != "DEADLINE_EXCEEDED" {
		t.Fatalf("record %+v", res.Record)
	}
}

func TestSend_GRPC_should_report_a_stream_timeout_and_keep_the_partial_messages(t *testing.T) {
	partial := grpcx.Result{Code: codes.OK, Messages: []transport.Message{
		{Direction: "sent", Body: []byte(`{}`)}, {Direction: "received", Body: []byte(`{"seq":1}`)}}}
	v := grpcVersion(model.GRPC{Receive: &model.AsyncReceive{Count: 3, Timeout: time.Second}})

	res, _ := sendGRPC(t, &scriptedGRPC{res: partial, err: transport.ErrTimeout}, v, nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAsyncTimeout) || res.Record == nil || res.Record.Outcome != OutcomeTimeout || len(res.Record.Messages) != 2 {
		t.Fatalf("status %s diags %v record %+v", res.Status, res.Diags, res.Record)
	}
}

func TestSend_GRPC_should_wire_live_messages_even_without_receive_options(t *testing.T) {
	f := newFixture(t)
	f.grpc = &scriptedGRPC{res: okOrder()}
	r := f.addRequest("orders/get", grpcVersion(model.GRPC{})) // no receive block

	_, events := f.send(f.input(r))

	live := 0
	for _, ev := range events {
		if _, ok := ev.(MessageReceived); ok {
			live++
		}
	}
	if live != 1 {
		t.Fatalf("%d messages were reported as they arrived, want 1: a watch must show its messages without waiting for the end", live)
	}
}

func TestSend_GRPC_should_keep_the_messages_received_when_the_send_is_cancelled(t *testing.T) {
	f := newFixture(t)
	f.grpc = &scriptedGRPC{hold: true, res: okOrder()}
	r := f.addRequest("orders/get", grpcVersion(model.GRPC{}))
	ctx, cancel := context.WithCancel(context.Background())
	s := StartSend(ctx, f.deps(), f.input(r))

	cancel()
	for range s.Events() {
	}
	res := s.Wait()

	if res.Status != StatusCancelled || res.Record == nil || res.Record.Outcome != OutcomeCancelled {
		t.Fatalf("status %s record %+v", res.Status, res.Record)
	}
	if got := len(res.Record.Messages); got != 2 {
		t.Fatalf("%d messages kept, want the request and the response", got)
	}
}

func TestSend_GRPC_should_give_a_stream_the_receive_options_and_a_deadline_to_match(t *testing.T) {
	runner := &scriptedGRPC{res: okOrder()}
	v := grpcVersion(model.GRPC{Receive: &model.AsyncReceive{Count: 3, Timeout: 7 * time.Second, Match: `body.id == "7"`}})

	sendGRPC(t, runner, v, nil)

	rc := runner.got.Receive
	if rc == nil || rc.Count != 3 || rc.Timeout != 7*time.Second || rc.Match == nil || runner.got.Timeout < 7*time.Second {
		t.Fatalf("receive %+v timeout %v", rc, runner.got.Timeout)
	}
}

func TestSend_GRPC_should_run_hooks_around_the_call(t *testing.T) {
	runner := &scriptedGRPC{res: okOrder()}
	v := grpcVersion(model.GRPC{Body: model.Body{JSON: `{"id":"{{vars.id}}"}`}})
	v.Hooks = model.Hooks{
		Pre:  []model.HookStep{inlineHook(setNode("s", "vars.id", "9"))},
		Post: []model.HookStep{inlineHook(assertNode("a", `code == "OK" and body.name == "order-7"`))},
	}

	res, _ := sendGRPC(t, runner, v, nil)

	if res.Status != StatusPassed || string(runner.got.JSON) != `{"id":"9"}` {
		t.Fatalf("status %s json %s diags %v", res.Status, runner.got.JSON, res.Diags)
	}
}

func TestSend_GRPC_should_not_leak_secrets(t *testing.T) {
	res0 := okOrder()
	res0.Messages[0].Headers = []transport.Header{{Name: "authorization", Value: "Bearer " + secretV}} // the sent message carries its metadata
	res0.Messages[1].Body = []byte(`{"echo":"` + secretV + `"}`)
	runner := &scriptedGRPC{res: res0}
	v := grpcVersion(model.GRPC{Server: "grpc://app:{{secret.password}}@api:50051", Body: model.Body{JSON: `{"pw":"{{secret.password}}"}`}})
	v.Headers = []model.KeyValue{{Key: "authorization", Value: "Bearer {{secret.password}}"}}

	res, _ := sendGRPC(t, runner, v, nil)

	if len(runner.got.Metadata) != 1 || runner.got.Metadata[0].Value != "Bearer "+secretV {
		t.Fatalf("the call must carry the real secret, got %+v", runner.got.Metadata)
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Fatalf("secret in result: %s", data)
	}
}

func TestSend_GRPC_should_not_record_a_password_written_in_the_server(t *testing.T) {
	v := grpcVersion(model.GRPC{Server: "grpc://app:plain-pw@api:50051"})

	res, _ := sendGRPC(t, &scriptedGRPC{res: okOrder()}, v, nil)

	data, _ := json.Marshal(res)
	if strings.Contains(string(data), "plain-pw") {
		t.Fatalf("password in result: %s", data)
	}
}

func TestSend_GRPC_should_turn_schema_and_body_errors_into_diagnostics(t *testing.T) {
	tests := map[string]struct {
		err  error
		want diag.Code
	}{
		"method":  {&grpcx.SchemaError{Detail: "method X is not in service Y"}, diag.CodeGRPCMethod},
		"proto":   {&grpcx.SchemaError{Detail: "compiling the .proto files: oops", Proto: true}, diag.CodeGRPCProto},
		"body":    {&grpcx.BadBodyError{Detail: `unknown field "idd"`}, diag.CodeGRPCBody},
		"general": {context.DeadlineExceeded, diag.CodeHTTPFailed},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res, _ := sendGRPC(t, &scriptedGRPC{err: tt.err}, grpcVersion(model.GRPC{}), nil)

			if res.Status != StatusError || !hasCode(res.Diags, tt.want) {
				t.Fatalf("status %s diags %v, want %s", res.Status, res.Diags, tt.want)
			}
		})
	}
}

func TestSend_GRPC_should_report_the_request_the_messages_and_the_outcome_as_events(t *testing.T) {
	f := newFixture(t)
	f.grpc = &scriptedGRPC{res: okOrder()}
	r := f.addRequest("orders/get", grpcVersion(model.GRPC{Receive: &model.AsyncReceive{}}))

	res, events := f.send(f.input(r))

	var sent *RequestSent
	var got *ResponseReceived
	messages := 0
	for _, ev := range events {
		switch e := ev.(type) {
		case RequestSent:
			sent = &e
		case ResponseReceived:
			got = &e
		case MessageReceived:
			messages++
		}
	}
	if res.Status != StatusPassed || sent == nil || got == nil || messages != 1 {
		t.Fatalf("status %s sent %v received %v messages %d", res.Status, sent, got, messages)
	}
	if sent.Method != "GRPC" || sent.URL != "grpc://api:50051/acme.Orders/Get" || got.Outcome != OutcomeOK {
		t.Fatalf("sent %+v received %+v", sent, got)
	}
}

func TestSend_GRPC_should_stop_when_cancelled(t *testing.T) {
	f := newFixture(t)
	f.grpc = &scriptedGRPC{hold: true}
	r := f.addRequest("orders/get", grpcVersion(model.GRPC{}))
	ctx, cancel := context.WithCancel(context.Background())
	s := StartSend(ctx, f.deps(), f.input(r))

	cancel()
	for range s.Events() {
	}

	if res := s.Wait(); res.Status != StatusCancelled {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_GRPC_should_save_a_record_that_loads_back(t *testing.T) {
	res, _ := sendGRPC(t, &scriptedGRPC{res: okOrder()}, grpcVersion(model.GRPC{}), func(in *SendInput) { in.Save = "per-status" })

	if len(res.Saved) != 2 || !strings.HasSuffix(res.Saved[0], "ok.json") {
		t.Fatalf("saved %v diags %v", res.Saved, res.Diags)
	}
	data, err := os.ReadFile(res.Saved[1])
	if err != nil {
		t.Fatal(err)
	}
	var back Record
	if err := json.Unmarshal(data, &back); err != nil || back.Kind != KindGRPC || back.GRPCCode != "OK" || len(back.Trailers) != 1 {
		t.Fatalf("loaded %+v err %v", back, err)
	}
}

func TestPreviewSend_GRPC_should_show_the_target_and_message_with_secrets_masked(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("orders/get", grpcVersion(model.GRPC{Body: model.Body{JSON: `{"pw":"{{secret.password}}","id":"{{id}}"}`}}))
	in := f.input(r)
	in.With = map[string]string{"id": "7"}

	p := PreviewSend(f.deps(), in, true)

	if p.Diags.HasErrors() || p.Masked.Method != "GRPC" || p.Masked.URL != "grpc://api:50051/acme.Orders/Get" || p.Masked.BodyKind != "text" {
		t.Fatalf("masked %+v diags %v", p.Masked, p.Diags)
	}
	if strings.Contains(p.Masked.Body, secretV) || !strings.Contains(p.Real.Body, secretV) {
		t.Fatalf("masked %q real %q", p.Masked.Body, p.Real.Body)
	}
}

func TestGRPCCallOf_should_render_the_call_for_listing_methods_without_sending_or_running_hooks(t *testing.T) {
	f := newFixture(t)
	v := grpcVersion(model.GRPC{Server: "grpc://{{var.host}}:50051", Proto: []string{"protos/o.proto"}, Import: []string{"protos"}})
	v.Variables = []model.KeyValue{{Key: "host", Value: "api"}}
	v.Headers = []model.KeyValue{{Key: "authorization", Value: "Bearer {{secret.password}}"}}
	v.Hooks = model.Hooks{Pre: []model.HookStep{inlineHook(assertNode("never", "false"))}}
	r := f.addRequest("orders/get", v)

	c, diags := GRPCCallOf(f.deps(), f.input(r), true)

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	if c.Server != "grpc://api:50051" || c.Root != f.coll().Root || len(c.Protos) != 1 || len(c.Metadata) != 1 || c.Metadata[0].Value != "Bearer "+secretV {
		t.Fatalf("call %+v", c)
	}
}

func TestGRPCCallOf_should_refuse_a_request_that_is_not_grpc(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("orders/http", get("/x"))

	_, diags := GRPCCallOf(f.deps(), f.input(r), false)

	if !hasCode(diags, diag.CodeGRPCRequest) {
		t.Fatalf("diags %v", diags)
	}
}

func TestSend_GRPC_should_pass_with_a_warning_when_a_field_of_the_message_was_left_out(t *testing.T) {
	res0 := okOrder()
	res0.Warnings = []string{"extra is not a field of shop.Q: it was left out"}

	res, _ := sendGRPC(t, &scriptedGRPC{res: res0}, grpcVersion(model.GRPC{}), nil)

	if res.Status != StatusPassed || !hasCode(res.Diags, diag.CodeGRPCField) || res.Diags.HasErrors() {
		t.Fatalf("status %s diags %v, want passed with a FLW-GRPC-006 warning", res.Status, res.Diags)
	}
}

func TestWithClientCert_should_add_the_certificate_a_mutual_TLS_block_names_and_report_a_missing_one(t *testing.T) {
	root := t.TempDir()
	certPEM, keyPEM := selfSignedPEM(t)
	for name, data := range map[string][]byte{"client.pem": certPEM, "client.key": keyPEM} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	base := &tls.Config{MinVersion: tls.VersionTLS12}
	good := &model.Auth{Type: model.AuthMutualTLS, Params: []model.KeyValue{{Key: "cert", Value: "client.pem"}, {Key: "key", Value: "client.key"}}}
	missing := &model.Auth{Type: model.AuthMutualTLS, Params: []model.KeyValue{{Key: "cert", Value: "gone.pem"}, {Key: "key", Value: "gone.key"}}}
	var okDiags, badDiags, otherDiags diag.List

	withCert := withClientCert(base, good, root, &okDiags)
	withClientCert(base, missing, root, &badDiags)
	untouched := withClientCert(base, &model.Auth{Type: model.AuthDigest}, root, &otherDiags)

	if len(okDiags) != 0 || len(withCert.Certificates) != 1 || len(base.Certificates) != 0 {
		t.Fatalf("diags %v, certificates %d (the shared config must stay as it was: %d)", okDiags, len(withCert.Certificates), len(base.Certificates))
	}
	if len(badDiags) != 1 || badDiags[0].Code != diag.CodeAuthInvalid {
		t.Fatalf("a missing certificate should be reported, got %v", badDiags)
	}
	if untouched != base || len(otherDiags) != 0 {
		t.Fatal("another scheme should leave the TLS settings alone")
	}
}

// selfSignedPEM makes a throwaway certificate and key in PEM.
func selfSignedPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "flowi-test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
