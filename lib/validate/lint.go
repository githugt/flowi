// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/credname"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// LintFinding is one suspicious property of a value. Char is set for
// invisible characters, as "U+XXXX".
type LintFinding struct {
	Code diag.Code
	Char string
}

// invisibleRunes are characters that look like nothing (or like a space)
// but change a value: they usually arrive by copy-paste from documents.
func invisibleRunes() []rune {
	return []rune{
		0x200B, // zero-width space
		0x200C, // zero-width non-joiner
		0x200D, // zero-width joiner
		0x2060, // word joiner
		0xFEFF, // byte order mark / zero-width no-break space
		0x00A0, // no-break space
	}
}

// LintValue reports leading/trailing whitespace, line breaks or tabs, and
// invisible characters in a single-line value.
func LintValue(value string) []LintFinding {
	var out []LintFinding
	if strings.TrimFunc(value, unicode.IsSpace) != value {
		out = append(out, LintFinding{Code: diag.CodeLintWhitespace})
	}
	if strings.ContainsAny(strings.TrimFunc(value, unicode.IsSpace), "\n\r\t") {
		out = append(out, LintFinding{Code: diag.CodeLintControl})
	}
	for _, r := range invisibleRunes() {
		if strings.ContainsRune(value, r) {
			out = append(out, LintFinding{Code: diag.CodeLintInvisible, Char: fmt.Sprintf("U+%04X", r)})
		}
	}
	return out
}

// lintSite is one value to lint, the field name shown to the user, and
// where the value lives.
type lintSite struct {
	file, path, field, value string
	// credential is the name to test for a credential (a header or query
	// parameter name, a variable name); empty for a site that is not one.
	credential string
}

func lintSites(c model.Collection) []lintSite {
	sites := []lintSite{{c.Manifest.File, "$.name", "name", c.Manifest.Name, ""}}
	for _, r := range c.Requests {
		sites = append(sites, lintSite{r.File, "$.name", "name", r.Name, ""})
		for _, v := range r.Versions {
			sites = append(sites, versionSites(r.File, v)...)
		}
	}
	for _, f := range c.Flows {
		sites = append(sites, lintSite{f.File, "$.name", "name", f.Name, ""})
	}
	for _, e := range c.Environments {
		for _, v := range e.Vars {
			credential := v.Name
			if v.Secret {
				credential = "" // a secret's value is in the vault, not in the file
			}
			sites = append(sites, lintSite{e.File, yamlpath.Key("$.vars", v.Name), "variable " + v.Name, v.Value, credential})
		}
	}
	return sites
}

func versionSites(file string, v model.RequestVersion) []lintSite {
	base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
	sites := []lintSite{{file, yamlpath.Key(base, "url"), "url", v.URL, ""}}
	headers := yamlpath.Key(base, "headers")
	for _, h := range v.Headers {
		if h.Disabled {
			continue
		}
		sites = append(sites,
			lintSite{file, headers, "header name " + h.Key, h.Key, ""},
			lintSite{file, yamlpath.Key(headers, h.Key), "header " + h.Key, h.Value, h.Key})
	}
	for _, q := range v.Query {
		if q.Disabled {
			continue
		}
		sites = append(sites, lintSite{file, yamlpath.Key(yamlpath.Key(base, "query"), q.Key), "query " + q.Key, q.Value, q.Key})
	}
	return sites
}

func checkLint(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		out = append(out, plainCredentials(c)...)
		out = append(out, plainGRPC(c)...)
	}
	reported := map[lintSite]bool{} // a global environment is in every collection: report it once
	for _, c := range ix.collections {
		for _, s := range lintSites(c) {
			if reported[s] {
				continue
			}
			reported[s] = true
			for _, f := range LintValue(s.value) {
				args := map[string]string{"field": s.field}
				if f.Char != "" {
					args["char"] = f.Char
				}
				out = append(out, diag.New(f.Code, diag.Location{File: s.file, Path: s.path}, args))
			}
			if typedCredential(s.credential, s.value) {
				out = append(out, diag.New(diag.CodeLintLiteralKey, diag.Location{File: s.file, Path: s.path}, map[string]string{"field": s.field}))
			}
		}
	}
	return out
}

// typedCredential reports a value written out in a file under a name that holds
// a credential. A value with a template ({{secret.x}}, {{env.x}}) is a reference,
// and a bare auth scheme ("Bearer") has no credential after it.
//
// Before: nothing warned when "Authorization: Bearer eyJ..." was typed into a
// request, and the token then reached git through the file.
func typedCredential(name, value string) bool {
	if name == "" || strings.Contains(value, "{{") || !credname.Sensitive(name) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "bearer", "basic", "digest", "token", "true", "false":
		return false
	}
	return true
}

// plainCredentials warns for a request version that sends a credential (a header or query parameter with a
// credential name, or user:password in the URL) over http or ws to a host that is not on this machine or a
// private network. A host written with a template is unknown and is left alone.
//
// Before: nothing warned when a token was sent in clear text across the internet.
func plainCredentials(c model.Collection) diag.List {
	var out diag.List
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			scheme, host, ok := plainRemote(v.URL)
			if !ok {
				continue
			}
			if !sendsCredential(v) {
				continue
			}
			base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
			out = append(out, diag.New(diag.CodeLintPlainKey, diag.Location{File: r.File, Path: yamlpath.Key(base, "url")},
				map[string]string{"field": "url", "scheme": scheme, "host": host}))
		}
	}
	return out
}

func sendsCredential(v model.RequestVersion) bool {
	for _, h := range v.Headers {
		if !h.Disabled && typedCredentialOrTemplate(h.Key, h.Value) {
			return true
		}
	}
	for _, q := range v.Query {
		if !q.Disabled && typedCredentialOrTemplate(q.Key, q.Value) {
			return true
		}
	}
	if u, err := url.Parse(v.URL); err == nil && u.User != nil {
		return true
	}
	return false
}

// typedCredentialOrTemplate is a credential-named field with something in it, typed or by reference.
func typedCredentialOrTemplate(name, value string) bool {
	return credname.Sensitive(name) && strings.TrimSpace(value) != ""
}

// plainRemote returns the scheme and host of a URL that is written in full as http or ws to a remote host.
func plainRemote(raw string) (scheme, host string, ok bool) {
	if before, _, found := strings.Cut(raw, "://"); !found || strings.Contains(before, "{{") || strings.Contains(hostPart(raw), "{{") {
		return "", "", false // not written in full: the scheme or the host is a template
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "ws") {
		return "", "", false
	}
	h := u.Hostname()
	if h == "" || strings.Contains(h, "{{") || isPrivateHost(h) {
		return "", "", false
	}
	return u.Scheme, h, true
}

// hostPart is the authority of raw, as far as it can be told.
func hostPart(raw string) string {
	_, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return ""
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// isPrivateHost is a name or address that stays on this machine or a private network.
func isPrivateHost(h string) bool {
	h = strings.ToLower(h)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || !strings.Contains(h, ".") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}

// plainGRPC warns for a gRPC call that goes without TLS to a host that is not on this machine or a private network.
// A scheme of grpc:// is plaintext by choice, and so is a bare address of a local host; a server written with a
// template is unknown. Before: a typo from grpcs:// to grpc:// silently changed the security of the call.
func plainGRPC(c model.Collection) diag.List {
	var out diag.List
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			if v.GRPC == nil || v.GRPC.Web || v.GRPC.Server == "" || strings.Contains(v.GRPC.Server, "{{") {
				continue
			}
			hostport, useTLS := model.GRPCTarget(v.GRPC.Server)
			host, _, err := net.SplitHostPort(hostport)
			if err != nil {
				host = hostport
			}
			if useTLS || host == "" || isPrivateHost(strings.Trim(host, "[]")) {
				continue
			}
			base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
			out = append(out, diag.New(diag.CodeLintPlainGRPC, diag.Location{File: r.File, Path: yamlpath.Key(yamlpath.Key(base, "grpc"), "server")},
				map[string]string{"host": host, "server": v.GRPC.Server}))
		}
	}
	return out
}
