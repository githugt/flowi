// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"net/http"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

// PreviewPart is one multipart field of a previewed request.
type PreviewPart struct {
	Name, Value, File, ContentType string
}

// PreviewRequest is a request rendered but not sent. BodyKind is "" (no
// body), "text" (JSON or text; Body is its text), "form" (Body is the
// encoded form), "multipart" (Parts) or "file" (File).
type PreviewRequest struct {
	Method   string
	URL      string
	Headers  []Header
	BodyKind string
	Body     string
	File     string
	Parts    []PreviewPart
}

// Preview is a request rendered without sending: no hooks run. Masked
// shows values that read a secret as vault.Mask. Real holds the actual
// values and is set only when they were asked for.
type Preview struct {
	Masked PreviewRequest
	Real   *PreviewRequest
	Diags  diag.List
}

// errPreviewLocked stops a preview from prompting for the vault: secrets
// stay unresolved (as written) unless exact values were asked for.
var errPreviewLocked = errors.New("previews do not unlock the vault")

// PreviewSend renders the request in in as a send would after its
// pre-hooks, without sending anything and without running hooks. Names
// that cannot be resolved (an unknown variable, an unknown generator, a
// secret while the vault is locked) stay as written rather than failing.
// Diags are problems that make the request unsendable, such as a body file
// outside the collection. includeSecrets reads the vault (which may
// prompt) and returns the real values in Real.
func PreviewSend(d Deps, in SendInput, includeSecrets bool) Preview {
	r := newRunState(d, in.Seed, nil)
	r.in, r.kind, r.env, r.prefer = in, "send", in.Env, in.PreferVersion
	if !includeSecrets {
		r.secrets.lockErr = errPreviewLocked
	}
	x, diags := r.prepare(sendCall(in))
	if diags.HasErrors() {
		return Preview{Diags: diags}
	}
	in2 := buildInput{
		version: x.version, scope: x.scope(false), empty: x.cfg.empty, label: x.c.request.Path,
		root: x.c.collection.Root, secretVars: x.env.secrets, missingSecret: x.r.secrets.isMissing(x.env.name),
		memo: x.memo, secret: x.r.secrets.derived, verbatim: true, defaults: x.r.defaultHeaders(x.cfg),
	}
	if x.version.GRPC != nil {
		return previewGRPC(in2, includeSecrets)
	}
	if x.version.Async != nil {
		return previewAsync(in2, includeSecrets)
	}
	b, diags := build(in2)
	out := Preview{Masked: previewOf(x.version.Body, b, false), Diags: diags}
	if includeSecrets {
		exact := previewOf(x.version.Body, b, true)
		out.Real = &exact
	}
	return out
}

func previewOf(body model.Body, b built, exact bool) PreviewRequest {
	p := PreviewRequest{Method: b.sent.Method, URL: b.sent.URL, Headers: b.sent.Header}
	if exact {
		p.URL, p.Headers = b.request.URL, realHeaders(b)
	}
	switch {
	case body.JSON != "" || body.Text != "":
		p.BodyKind, p.Body = "text", maskedBody(b, exact)
	case len(body.Form) > 0:
		p.BodyKind, p.Body = "form", maskedBody(b, exact)
	case len(body.Multipart) > 0:
		p.BodyKind, p.Parts = "multipart", previewParts(b, exact)
	case body.File != "":
		p.BodyKind = "file"
		if b.sent.Body != nil {
			p.File = b.sent.Body.File
		}
	}
	return p
}

func maskedBody(b built, exact bool) string {
	if exact {
		return b.bodyText
	}
	if b.sent.Body == nil {
		return ""
	}
	return b.sent.Body.Text
}

func previewParts(b built, exact bool) []PreviewPart {
	out := make([]PreviewPart, len(b.parts))
	for i, p := range b.parts {
		out[i] = PreviewPart{Name: p.Name, Value: p.Value, File: p.File, ContentType: p.ContentType}
		if !exact && b.tainted["body"] {
			out[i].Value, out[i].File = maskIfSet(p.Value), maskIfSet(p.File)
		}
	}
	return out
}

func maskIfSet(s string) string {
	if s == "" {
		return ""
	}
	return vault.Mask
}

// realHeaders are the request's actual header values, in the order the
// masked form lists them.
func realHeaders(b built) []Header {
	seen := map[string]int{}
	out := make([]Header, 0, len(b.sent.Header))
	for _, h := range b.sent.Header {
		values := http.Header(b.request.Header).Values(h.Name)
		i := seen[h.Name]
		seen[h.Name]++
		if i < len(values) {
			out = append(out, Header{Name: h.Name, Value: values[i]})
		}
	}
	return out
}

// previewAsync renders an async version: the protocol stands in for the
// method, and the server and channel for the URL.
func previewAsync(in buildInput, includeSecrets bool) Preview {
	b, diags := buildAsync(in)
	masked := PreviewRequest{Method: b.sent.Method, URL: b.sent.URL, Headers: b.sent.Header}
	if b.sent.Body != nil {
		masked.BodyKind, masked.Body = "text", b.sent.Body.Text
	}
	out := Preview{Masked: masked, Diags: diags}
	if includeSecrets {
		exact := PreviewRequest{Method: b.sent.Method, URL: model.TargetURL(b.protocol, b.op.Server, b.op.Channel), Headers: b.sent.Header}
		if b.op.Send != nil {
			exact.BodyKind, exact.Body = "text", string(b.op.Send.Body)
		}
		out.Real = &exact
	}
	return out
}
