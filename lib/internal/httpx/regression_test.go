// SPDX-License-Identifier: Apache-2.0

package httpx

// Regression tests for bug classes other API clients have had. A failing
// test here is a real gap in flowi-lib, not a broken test.

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func send(t *testing.T, c *Client, r Request) Attempt {
	t.Helper()
	attempts, err := c.Do(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return attempts[len(attempts)-1]
}

// otherHostURL rewrites a 127.0.0.1 test-server URL to "localhost", which
// is a different host to the redirect policy of net/http (it ignores ports).
func otherHostURL(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	u.Host = "localhost:" + u.Port()
	return u.String()
}

// Auth headers leaked on cross-origin redirects.
// net/http strips Authorization and Cookie itself, but not custom
// credential headers such as X-Api-Key.
func TestClient_Given_cross_host_redirect_When_following_Then_credential_headers_are_not_forwarded(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"authorization", "Authorization"},
		{"api key header", "X-Api-Key"},
		{"token header", "X-Auth-Token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get(tc.header)
			}))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, otherHostURL(t, target.URL), http.StatusFound)
			}))
			defer origin.Close()
			c := newClient(t, defaultOptions())

			a := send(t, c, Request{Method: http.MethodGet, URL: origin.URL, Header: http.Header{tc.header: {"secret-value"}}})

			if a.Err != nil {
				t.Fatal(a.Err)
			}
			if got != "" {
				t.Fatalf("%s reached another host: %q", tc.header, got)
			}
		})
	}
}

// Cookies were not stored when redirects were off.
func TestJar_Given_redirects_off_When_302_sets_cookie_Then_cookie_is_stored_and_sent_next(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		if c, err := r.Cookie("session"); err == nil {
			_, _ = io.WriteString(w, c.Value)
		}
	}))
	defer srv.Close()
	o := defaultOptions()
	o.FollowRedirects = false
	o.Jar = NewJar()
	c := newClient(t, o)

	first := send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/login", Header: http.Header{}})
	second := send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/home", Header: http.Header{}})

	if first.Response == nil || first.Response.Status != http.StatusFound {
		t.Fatalf("want the 302 itself, got %+v", first)
	}
	if string(second.Response.Body) != "abc" {
		t.Fatalf("cookie from the 302 was not kept: %q", second.Response.Body)
	}
}

// Cookies from 4xx and 5xx responses were dropped.
func TestJar_Given_error_status_When_response_sets_cookie_Then_cookie_is_kept(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/fail" {
					http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "t1", Path: "/"})
					w.WriteHeader(status)
					return
				}
				if c, err := r.Cookie("csrf"); err == nil {
					_, _ = io.WriteString(w, c.Value)
				}
			}))
			defer srv.Close()
			o := defaultOptions()
			o.Jar = NewJar()
			c := newClient(t, o)

			send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/fail", Header: http.Header{}})
			next := send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/next", Header: http.Header{}})

			if string(next.Response.Body) != "t1" {
				t.Fatalf("cookie lost: %q", next.Response.Body)
			}
		})
	}
}

// __Host- prefixed cookies must be storable.
func TestJar_Given_host_prefixed_cookie_When_set_over_http_Then_behaviour_is_documented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "__Host-id", Value: "1", Path: "/", Secure: true})
			return
		}
		if c, err := r.Cookie("__Host-id"); err == nil {
			_, _ = io.WriteString(w, c.Value)
		}
	}))
	defer srv.Close()
	o := defaultOptions()
	o.Jar = NewJar()
	c := newClient(t, o)

	send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/set", Header: http.Header{}})
	got := send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/get", Header: http.Header{}})

	// A Secure cookie is not sent over plain http by browsers; this only
	// records what the jar does so a change is noticed.
	t.Logf("__Host- cookie over plain http came back as %q", got.Response.Body)
}

// A user-defined multipart boundary was overwritten.
// Here the caller sets Content-Type with its own boundary, but the body
// writer picks a random one, so the server cannot split the parts.
func TestMultipartBody_Given_user_content_type_with_boundary_When_sent_Then_server_can_parse_parts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("file-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	var parseErr error
	var fields map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			parseErr = err
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields = map[string]string{}
		for {
			p, err := mr.NextPart()
			if err != nil {
				if err != io.EOF {
					parseErr = err
				}
				return
			}
			b, _ := io.ReadAll(p)
			fields[p.FormName()] = string(b)
		}
	}))
	defer srv.Close()
	body, err := MultipartBody(dir, []Part{{Name: "k", Value: "v"}, {Name: "f", File: "a.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	c := newClient(t, defaultOptions())

	a := send(t, c, Request{
		Method: http.MethodPost, URL: srv.URL, Body: body,
		Header: http.Header{"Content-Type": {"multipart/form-data; boundary=my-own-boundary"}},
	})

	if a.Err != nil {
		t.Fatal(a.Err)
	}
	if parseErr != nil || fields["k"] != "v" || fields["f"] != "file-data" {
		t.Fatalf("server could not read parts: err=%v fields=%v", parseErr, fields)
	}
}

// Media types must match ignoring parameters and case.
func TestContentTypeOf_Given_mixed_case_and_params_When_detected_Then_file_part_type_is_set(t *testing.T) {
	tests := map[string]string{
		"a.JSON": "application/json",
		"b.json": "application/json",
	}
	for name, wantPrefix := range tests {
		got := contentTypeOf(name)
		if !strings.HasPrefix(strings.ToLower(got), wantPrefix) {
			t.Errorf("contentTypeOf(%q) = %q, want prefix %q", name, got, wantPrefix)
		}
	}
}

// A header row with an empty name crashed
// the CLI. A send must fail or skip it, never panic.
func TestClient_Given_empty_header_name_When_sent_Then_no_panic_and_outcome_is_reported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	c := newClient(t, defaultOptions())

	a := send(t, c, Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{"": {"x"}, "X-Ok": {"1"}}})

	if a.Err == nil && a.Response == nil {
		t.Fatal("no outcome reported")
	}
	t.Logf("empty header name: err=%v", a.Err)
}

// .local names must resolve through the operating
// system, which is the only resolver that speaks mDNS. This registers a
// throwaway mDNS host with dns-sd and sends a request to it. It runs on
// macOS only; on Linux, a pure-Go resolver (any CGO_ENABLED=0 build) does
// not query mDNS, which this test cannot check from here.
func TestClient_Given_mdns_only_host_When_sent_Then_name_resolves_through_the_system(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("registers an mDNS host with dns-sd, which is macOS only")
	}
	dnsSD, err := exec.LookPath("dns-sd")
	if err != nil {
		t.Skip("dns-sd not installed")
	}
	ip := lanIPv4(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "mdns-ok")
	}))
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = l
	srv.Start()
	defer srv.Close()
	port := l.Addr().(*net.TCPAddr).Port
	host := "flowi-mdns-test.local"
	reg := exec.CommandContext(t.Context(), dnsSD, "-P", "flowitest", "_http._tcp", "local", fmt.Sprint(port), host, ip)
	if err := reg.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reg.Process.Kill(); _ = reg.Wait() }() // the registration process is ours; its exit status is a kill
	time.Sleep(2 * time.Second)                               // mDNSResponder needs a moment to publish the record
	c := newClient(t, defaultOptions())

	a := send(t, c, Request{Method: http.MethodGet, URL: fmt.Sprintf("http://%s:%d/", host, port), Header: http.Header{}})

	if a.Err != nil || string(a.Response.Body) != "mdns-ok" {
		t.Fatalf("mDNS host did not resolve: err=%v", a.Err)
	}
}

func lanIPv4(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			return n.IP.String()
		}
	}
	t.Skip("no LAN address to publish")
	return ""
}

// A .local name that nothing answers must fail as a transport error, not
// hang until the request timeout.
func TestClient_Given_unresolvable_local_host_When_sent_Then_error_is_a_transport_error(t *testing.T) {
	o := defaultOptions()
	o.Timeout = 10 * time.Second
	c := newClient(t, o)
	start := time.Now()

	a := send(t, c, Request{Method: http.MethodGet, URL: "http://flowi-nonexistent.local/", Header: http.Header{}})

	if a.Err == nil {
		t.Fatal("expected an error")
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("took %s to give up", elapsed)
	}
}

// Same-origin redirects keep headers; the strip applies only when the
// request leaves the origin.
func TestClient_Given_same_origin_redirect_When_following_Then_headers_are_kept(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusFound)
			return
		}
		got = r.Header.Get("X-Api-Key")
	}))
	defer srv.Close()
	c := newClient(t, defaultOptions())

	send(t, c, Request{Method: http.MethodGet, URL: srv.URL + "/start", Header: http.Header{"X-Api-Key": {"k"}}})

	if got != "k" {
		t.Fatalf("header lost on same-origin redirect: %q", got)
	}
}

// A different port is a different origin; net/http alone would forward
// everything except Authorization and Cookie.
func TestClient_Given_redirect_to_other_port_When_following_Then_credential_headers_are_not_forwarded(t *testing.T) {
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Api-Key") + r.Header.Get("Authorization")
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer origin.Close()
	c := newClient(t, defaultOptions())

	send(t, c, Request{Method: http.MethodGet, URL: origin.URL, Header: http.Header{"X-Api-Key": {"k"}, "Authorization": {"Bearer t"}}})

	if got != "" {
		t.Fatalf("credentials reached another port: %q", got)
	}
}

// Headers that carry no credential survive a cross-origin redirect.
func TestClient_Given_cross_host_redirect_When_following_Then_plain_headers_are_kept(t *testing.T) {
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Trace-Id")
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherHostURL(t, target.URL), http.StatusFound)
	}))
	defer origin.Close()
	c := newClient(t, defaultOptions())

	send(t, c, Request{Method: http.MethodGet, URL: origin.URL, Header: http.Header{"X-Trace-Id": {"abc"}}})

	if got != "abc" {
		t.Fatalf("plain header dropped: %q", got)
	}
}

// A multipart Content-Type without a boundary cannot be parsed by any
// server; the body's own boundary is added to it.
func TestMultipartBody_Given_user_multipart_type_without_boundary_When_sent_Then_boundary_is_added(t *testing.T) {
	var contentType string
	var fields map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		_, params, _ := mime.ParseMediaType(contentType)
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields = map[string]string{}
		for {
			p, err := mr.NextPart()
			if err != nil {
				return
			}
			b, _ := io.ReadAll(p)
			fields[p.FormName()] = string(b)
		}
	}))
	defer srv.Close()
	body, err := MultipartBody(t.TempDir(), []Part{{Name: "k", Value: "v"}})
	if err != nil {
		t.Fatal(err)
	}
	c := newClient(t, defaultOptions())

	send(t, c, Request{Method: http.MethodPost, URL: srv.URL, Body: body, Header: http.Header{"Content-Type": {"multipart/mixed"}}})

	if !strings.HasPrefix(contentType, "multipart/mixed") || fields["k"] != "v" {
		t.Fatalf("content type %q, fields %v", contentType, fields)
	}
}

// Before: a boundary the multipart writer rejects (here, empty after
// trimming is invalid, a boundary with a bad character is) was sent as is.
func TestMultipartBody_Given_invalid_user_boundary_When_sent_Then_attempt_fails_with_a_clear_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	body, err := MultipartBody(t.TempDir(), []Part{{Name: "k", Value: "v"}})
	if err != nil {
		t.Fatal(err)
	}
	c := newClient(t, defaultOptions())

	a := send(t, c, Request{Method: http.MethodPost, URL: srv.URL, Body: body, Header: http.Header{"Content-Type": {`multipart/form-data; boundary="bad boundary!"`}}})

	if a.Err == nil || !strings.Contains(a.Err.Error(), "boundary") {
		t.Fatalf("err = %v", a.Err)
	}
}

// redirectRig serves a chain: origin (127.0.0.1) redirects to target
// (localhost, another host), which records the credential headers it got.
type redirectRig struct {
	origin, target  *httptest.Server
	gotKey, gotAuth string
}

func newRedirectRig(t *testing.T, targetURL func(*testing.T, string) string) *redirectRig {
	t.Helper()
	rig := &redirectRig{}
	rig.target = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		rig.gotKey, rig.gotAuth = r.Header.Get("X-Api-Key"), r.Header.Get("Authorization")
	}))
	rig.origin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetURL(t, rig.target.URL), http.StatusFound)
	}))
	t.Cleanup(func() { rig.origin.Close(); rig.target.Close() })
	return rig
}

func (rig *redirectRig) send(t *testing.T, o Options) Attempt {
	t.Helper()
	c := newClient(t, o)
	return send(t, c, Request{Method: http.MethodGet, URL: rig.origin.URL, Header: http.Header{"X-Api-Key": {"k"}, "Authorization": {"Bearer t"}}})
}

func TestClient_Given_redirect_credentials_mode_When_redirect_leaves_host_Then_headers_follow_the_mode(t *testing.T) {
	tests := []struct {
		name         string
		mode         string
		keepHeaders  []string
		wantKey      string
		wantAuth     string
		wantForwards []string
	}{
		{"default strips both", "", nil, "", "", nil},
		{"strip drops both", RedirectStrip, nil, "", "", nil},
		{"keep forwards both, restoring Authorization", RedirectKeep, nil, "k", "Bearer t", []string{"Authorization", "X-Api-Key"}},
		{"allow-list forwards one under strip", RedirectStrip, []string{"x-api-key"}, "k", "", []string{"X-Api-Key"}},
		{"allow-list can name Authorization", RedirectStrip, []string{"Authorization"}, "", "Bearer t", []string{"Authorization"}},
		{"same_site drops across unrelated hosts", RedirectSameSite, nil, "", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rig := newRedirectRig(t, otherHostURL)
			o := defaultOptions()
			o.RedirectCredentials, o.RedirectKeepHeaders = tc.mode, tc.keepHeaders

			a := rig.send(t, o)

			if a.Err != nil {
				t.Fatal(a.Err)
			}
			if rig.gotKey != tc.wantKey || rig.gotAuth != tc.wantAuth {
				t.Fatalf("target got key=%q auth=%q", rig.gotKey, rig.gotAuth)
			}
			hops := a.Response.Redirects
			if len(hops) != 1 || !slices.Equal(hops[0].Forwarded, tc.wantForwards) {
				t.Fatalf("hops %+v, want forwarded %v", hops, tc.wantForwards)
			}
		})
	}
}

func TestSameSite_Given_urls_When_compared_Then_registrable_domain_decides(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://api.example.com/", "https://www.example.com/", true},
		{"https://example.com/", "https://example.org/", false},
		{"https://a.github.io/", "https://b.github.io/", false}, // github.io is a public suffix
		{"http://127.0.0.1:1/", "http://127.0.0.1:2/", true},
		{"http://127.0.0.1/", "http://localhost/", false},
		{"https://a.example.co.uk/", "https://b.example.co.uk/", true},
	}
	for _, tc := range tests {
		if got := sameSite(mustURL(t, tc.a), mustURL(t, tc.b)); got != tc.want {
			t.Errorf("sameSite(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestApplyCredentialPolicy_Given_same_site_mode_When_subdomain_changes_Then_kept_unless_downgraded(t *testing.T) {
	first := func(raw string) *http.Request {
		r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, raw, nil)
		r.Header.Set("Authorization", "Bearer t")
		r.Header.Set("X-Api-Key", "k")
		return r
	}
	next := func(raw string) *http.Request {
		r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, raw, nil)
		r.Header.Set("X-Api-Key", "k") // net/http copies custom headers and drops Authorization across hosts
		return r
	}
	o := Options{RedirectCredentials: RedirectSameSite}

	kept := next("https://www.example.com/")
	applyCredentialPolicy(o, first("https://api.example.com/"), kept)
	downgraded := next("http://www.example.com/")
	applyCredentialPolicy(o, first("https://api.example.com/"), downgraded)

	if kept.Header.Get("Authorization") != "Bearer t" || kept.Header.Get("X-Api-Key") != "k" {
		t.Errorf("same-site redirect lost credentials: %v", kept.Header)
	}
	if downgraded.Header.Get("Authorization") != "" || downgraded.Header.Get("X-Api-Key") != "" {
		t.Errorf("https to http kept credentials: %v", downgraded.Header)
	}
}

// net/http copies the first request's headers again on every hop and only
// compares with the hop before, so the policy has to measure from the first
// request or the second hop on the other host would get the headers back.
func TestClient_Given_two_hops_on_the_other_host_When_following_Then_headers_stay_stripped(t *testing.T) {
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/second", http.StatusFound)
			return
		}
		got = r.Header.Get("X-Api-Key")
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherHostURL(t, target.URL), http.StatusFound)
	}))
	defer origin.Close()
	c := newClient(t, defaultOptions())

	send(t, c, Request{Method: http.MethodGet, URL: origin.URL, Header: http.Header{"X-Api-Key": {"k"}}})

	if got != "" {
		t.Fatalf("second hop got the key back: %q", got)
	}
}
