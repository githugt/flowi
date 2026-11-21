// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/githugt/flowi/lib/engine"
)

// The formats that are code or a file rather than a shell command: a .http request, JavaScript fetch and
// Python requests. Before: only four shell tools were offered, so a request could not be shared as a .http file
// or pasted into a script.

// quote writes s as a double-quoted string literal that JavaScript and Python both read: JSON's own escapes,
// without the HTML escaping (a URL with a query keeps its &).
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(b.String(), "\n")
}

func method(r engine.PreviewRequest) string {
	if r.Method == "" {
		return http.MethodGet
	}
	return r.Method
}

// codeHeaders are the headers of r for a snippet. A multipart request leaves its own Content-Type out: the boundary
// is written by whoever sends it (the .http lines below, fetch, requests).
func codeHeaders(r engine.PreviewRequest) []engine.Header {
	var out []engine.Header
	for _, h := range r.Headers {
		if r.BodyKind == "multipart" && strings.EqualFold(h.Name, "Content-Type") {
			continue
		}
		out = append(out, h)
	}
	return out
}

// httpFile is a request in the .http format of editors and tools (request line, headers, a blank line, the body).
func httpFile(r engine.PreviewRequest) string {
	var b strings.Builder
	b.WriteString(method(r) + " " + r.URL + "\n")
	for _, h := range codeHeaders(r) {
		b.WriteString(h.Name + ": " + h.Value + "\n")
	}
	switch r.BodyKind {
	case "text", "form":
		b.WriteString("\n" + r.Body + "\n")
	case "file":
		b.WriteString("\n< " + r.File + "\n")
	case "multipart":
		const boundary = "flowi-boundary"
		b.WriteString("Content-Type: multipart/form-data; boundary=" + boundary + "\n\n")
		for _, p := range r.Parts {
			b.WriteString("--" + boundary + "\n")
			if p.File == "" {
				b.WriteString("Content-Disposition: form-data; name=\"" + p.Name + "\"\n\n" + p.Value + "\n")
				continue
			}
			b.WriteString("Content-Disposition: form-data; name=\"" + p.Name + "\"; filename=\"" + p.File + "\"\n")
			if p.ContentType != "" {
				b.WriteString("Content-Type: " + p.ContentType + "\n")
			}
			b.WriteString("\n< " + p.File + "\n")
		}
		b.WriteString("--" + boundary + "--\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// fetchJS is a request as a call of fetch (browsers, Node 18+, Deno, Bun). A file is read with fs.openAsBlob (Node 20).
func fetchJS(r engine.PreviewRequest) string {
	var b strings.Builder
	multipart := r.BodyKind == "multipart"
	if multipart {
		b.WriteString("const form = new FormData()\n")
		for _, p := range r.Parts {
			if p.File == "" {
				b.WriteString("form.append(" + quote(p.Name) + ", " + quote(p.Value) + ")\n")
			} else {
				b.WriteString("form.append(" + quote(p.Name) + ", await (await import('node:fs')).openAsBlob(" + quote(p.File) + "))\n")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("const response = await fetch(" + quote(r.URL) + ", {\n")
	b.WriteString("  method: " + quote(method(r)) + ",\n")
	if hs := codeHeaders(r); len(hs) > 0 {
		b.WriteString("  headers: {\n")
		for _, h := range hs {
			b.WriteString("    " + quote(h.Name) + ": " + quote(h.Value) + ",\n")
		}
		b.WriteString("  },\n")
	}
	switch r.BodyKind {
	case "text", "form":
		b.WriteString("  body: " + quote(r.Body) + ",\n")
	case "file":
		b.WriteString("  body: await (await import('node:fs')).openAsBlob(" + quote(r.File) + "),\n")
	case "multipart":
		b.WriteString("  body: form,\n")
	}
	b.WriteString("})")
	return b.String()
}

// pythonRequests is a request as a call of requests.request.
func pythonRequests(r engine.PreviewRequest) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	b.WriteString("response = requests.request(\n")
	b.WriteString("    " + quote(method(r)) + ",\n")
	b.WriteString("    " + quote(r.URL) + ",\n")
	if hs := codeHeaders(r); len(hs) > 0 {
		b.WriteString("    headers={\n")
		for _, h := range hs {
			b.WriteString("        " + quote(h.Name) + ": " + quote(h.Value) + ",\n")
		}
		b.WriteString("    },\n")
	}
	switch r.BodyKind {
	case "text", "form":
		b.WriteString("    data=" + quote(r.Body) + ".encode(),\n")
	case "file":
		b.WriteString("    data=open(" + quote(r.File) + ", \"rb\"),\n")
	case "multipart":
		b.WriteString("    files={\n")
		for _, p := range r.Parts {
			if p.File == "" {
				b.WriteString("        " + quote(p.Name) + ": (None, " + quote(p.Value) + "),\n")
				continue
			}
			ct := ""
			if p.ContentType != "" {
				ct = ", " + quote(p.ContentType)
			}
			b.WriteString("        " + quote(p.Name) + ": (" + quote(p.File) + ", open(" + quote(p.File) + ", \"rb\")" + ct + "),\n")
		}
		b.WriteString("    },\n")
	}
	b.WriteString(")")
	return b.String()
}
