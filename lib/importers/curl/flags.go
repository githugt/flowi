// SPDX-License-Identifier: Apache-2.0

package curl

import (
	"strings"

	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

// flagSpec says whether a flag takes a value and what it does; a nil set
// means the flag is understood and has no meaning in a saved request.
type flagSpec struct {
	arg bool
	set func(b *builder, value token)
}

// flags is curl's flag table as far as an imported request is concerned.
func flags() map[string]flagSpec {
	method := flagSpec{arg: true, set: func(b *builder, v token) { b.req.method = v.text }}
	hdr := flagSpec{arg: true, set: (*builder).addHeader}
	data := func(name string) flagSpec {
		return flagSpec{arg: true, set: func(b *builder, v token) { b.addData(name, v) }}
	}
	urlencode := flagSpec{arg: true, set: func(b *builder, v token) {
		k, val, ok := strings.Cut(v.text, "=")
		if !ok || k == "" {
			b.req.data = append(b.req.data, v.text)
			return
		}
		b.req.dataForm = append(b.req.dataForm, model.KeyValue{Key: k, Value: val})
	}}
	form := flagSpec{arg: true, set: (*builder).addForm}
	user := flagSpec{arg: true, set: func(b *builder, _ token) {
		b.req.headers = append(b.req.headers, header{name: "Authorization", value: ""})
	}}
	cookie := flagSpec{arg: true, set: func(b *builder, v token) {
		if !strings.Contains(v.text, "=") {
			b.it.Unmapped("cookies from file "+v.text, "left out", "Flow-I keeps cookies in its own jar")
			return
		}
		b.req.headers = append(b.req.headers, header{name: "Cookie", value: v.text, at: v})
	}}
	named := func(name string) flagSpec {
		return flagSpec{arg: true, set: func(b *builder, v token) {
			b.req.headers = append(b.req.headers, header{name: name, value: v.text, at: v})
		}}
	}
	// unsupported reports the flag, never its value (it may be a secret).
	unsupported := func(what string) flagSpec {
		return flagSpec{arg: true, set: func(b *builder, v token) {
			b.it.UnmappedAt(b.at(v.offset), what, "ignored", "set it in Flow-I's settings instead")
		}}
	}
	bearer := flagSpec{arg: true, set: func(b *builder, _ token) {
		b.req.headers = append(b.req.headers, header{name: "Authorization", value: ""})
	}}
	ignored, ignoredArg := flagSpec{}, flagSpec{arg: true}
	t := map[string]flagSpec{
		"-X": method, "--request": method,
		"-H": hdr, "--header": hdr,
		"-d": data("-d"), "--data": data("--data"), "--data-ascii": data("--data-ascii"),
		"--data-binary": data("--data-binary"), "--data-raw": data("--data-raw"),
		"--data-urlencode": urlencode,
		"--json":           {arg: true, set: func(b *builder, v token) { b.req.json = v.text }},
		"-F":               form, "--form": form, "--form-string": form,
		"-u": user, "--user": user,
		"-b": cookie, "--cookie": cookie,
		"-A": named("User-Agent"), "--user-agent": named("User-Agent"),
		"-e": named("Referer"), "--referer": named("Referer"),
		"-G": {set: func(b *builder, _ token) { b.req.get = true }}, "--get": {set: func(b *builder, _ token) { b.req.get = true }},
		"-I": {set: func(b *builder, _ token) { b.req.head = true }}, "--head": {set: func(b *builder, _ token) { b.req.head = true }},
		"--url": {arg: true, set: (*builder).setURL},
		"-T":    {arg: true, set: func(b *builder, v token) { b.req.upload = v.text }}, "--upload-file": {arg: true, set: func(b *builder, v token) { b.req.upload = v.text }},
		"-x": unsupported("proxy"), "--proxy": unsupported("proxy"),
		"-E": unsupported("client certificate"), "--cert": unsupported("client certificate"),
		"--key": unsupported("client key"), "--cacert": unsupported("CA certificate"),
		"--oauth2-bearer": bearer,
		"-U":              unsupported("proxy credentials"), "--proxy-user": unsupported("proxy credentials"),
		"--aws-sigv4": unsupported("AWS signing"),
	}
	for _, f := range []string{"-s", "--silent", "-S", "--show-error", "-k", "--insecure", "-L", "--location", "-v", "--verbose",
		"-i", "--include", "--compressed", "-f", "--fail", "-N", "--no-buffer", "-O", "--remote-name", "-#", "--progress-bar",
		"--http1.1", "--http2", "--http2-prior-knowledge", "-g", "--globoff", "-n", "--netrc", "--location-trusted", "-j",
		"--digest", "--ntlm", "--negotiate", "--anyauth", "--basic"} {
		t[f] = ignored
	}
	for _, f := range []string{"-o", "--output", "-m", "--max-time", "--connect-timeout", "--retry", "--retry-delay",
		"--retry-max-time", "-w", "--write-out", "-c", "--cookie-jar", "--resolve", "--limit-rate", "--max-redirs", "-K", "--config"} {
		t[f] = ignoredArg
	}
	return t
}

// addHeader reads "Name: value"; "Name;" is curl's way to send it empty.
func (b *builder) addHeader(v token) {
	name, value, ok := strings.Cut(v.text, ":")
	switch {
	case ok:
		value = strings.TrimSpace(value)
	case strings.HasSuffix(v.text, ";"):
		name = strings.TrimSuffix(v.text, ";")
	default:
		b.it.Unmapped("header "+v.text, "ignored", "curl removes a header written without a colon")
		return
	}
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, "Content-Length") || strings.EqualFold(name, "Host") {
		return // computed when sending
	}
	b.req.headers = append(b.req.headers, header{name: name, value: value, at: v})
}

// addForm reads -F name=value or name=@file[;type=…].
func (b *builder) addForm(v token) {
	name, value, ok := strings.Cut(v.text, "=")
	if !ok {
		b.it.Unmapped("form part "+v.text, "ignored", "a form part needs name=value")
		return
	}
	if file, isFile := strings.CutPrefix(value, "@"); isFile {
		path, _, _ := strings.Cut(file, ";")
		b.req.form = append(b.req.form, model.Field{Key: name, Value: []model.Field{{Key: "file", Value: importers.Literal(path)}}})
		b.it.Unmapped("file part "+name, "kept with its path", "the path is relative to where curl ran")
		return
	}
	if importers.SensitiveHeader(name) && value != "" {
		value = b.it.Redact("form field "+name, name)
	} else {
		value = importers.Literal(value)
	}
	b.req.form = append(b.req.form, model.Field{Key: name, Value: value})
}
