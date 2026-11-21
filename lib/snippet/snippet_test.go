// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func jsonPost() engine.PreviewRequest {
	return engine.PreviewRequest{
		Method: "POST", URL: "https://api.example.com/v1/users?x=1",
		Headers:  []engine.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Authorization", Value: "Bearer it's"}},
		BodyKind: "text", Body: `{"name":"Ada"}`,
	}
}

func TestGenerate_should_render_each_format_for_a_json_post(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{Curl, "curl -X POST 'https://api.example.com/v1/users?x=1' \\\n  -H 'Content-Type: application/json' \\\n  -H 'Authorization: Bearer it'\\''s' \\\n  --data-raw '{\"name\":\"Ada\"}'"},
		{Wget, "wget -O - --method=POST \\\n  --header='Content-Type: application/json' \\\n  --header='Authorization: Bearer it'\\''s' \\\n  --body-data='{\"name\":\"Ada\"}' \\\n  'https://api.example.com/v1/users?x=1'"},
		{PowerShell, "Invoke-RestMethod -Method Post -Uri 'https://api.example.com/v1/users?x=1' `\n  -ContentType 'application/json' `\n  -Headers @{ 'Authorization' = 'Bearer it''s' } `\n  -Body '{\"name\":\"Ada\"}'"},
		{Xh, "xh POST 'https://api.example.com/v1/users?x=1' \\\n  'Content-Type:application/json' \\\n  'Authorization:Bearer it'\\''s' \\\n  --raw '{\"name\":\"Ada\"}'"},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got, err := Generate(tt.format, jsonPost())

			if err != nil || got != tt.want {
				t.Fatalf("err %v\n--- got\n%s\n--- want\n%s", err, got, tt.want)
			}
		})
	}
}

func TestGenerate_should_omit_the_method_for_a_plain_get(t *testing.T) {
	r := engine.PreviewRequest{Method: "GET", URL: "https://x.test/"}

	curl, _ := Generate(Curl, r)
	xh, _ := Generate(Xh, r)
	ps, _ := Generate(PowerShell, r)

	if curl != "curl 'https://x.test/'" || xh != "xh 'https://x.test/'" || ps != "Invoke-RestMethod -Method Get -Uri 'https://x.test/'" {
		t.Fatalf("curl %q xh %q ps %q", curl, xh, ps)
	}
}

func TestGenerate_should_describe_multipart_and_file_bodies(t *testing.T) {
	multi := engine.PreviewRequest{Method: "POST", URL: "http://x", BodyKind: "multipart",
		Parts: []engine.PreviewPart{{Name: "n", Value: "v"}, {Name: "f", File: "a.png", ContentType: "image/png"}}}
	file := engine.PreviewRequest{Method: "PUT", URL: "http://x", BodyKind: "file", File: "payload.bin"}

	curlMulti, _ := Generate(Curl, multi)
	curlFile, _ := Generate(Curl, file)
	xhMulti, _ := Generate(Xh, multi)
	xhFile, _ := Generate(Xh, file)
	psMulti, _ := Generate(PowerShell, multi)
	wgetMulti, _ := Generate(Wget, multi)

	if !strings.Contains(curlMulti, "-F 'n=v'") || !strings.Contains(curlMulti, "-F 'f=@a.png;type=image/png'") {
		t.Fatalf("curl multipart:\n%s", curlMulti)
	}
	if !strings.Contains(curlFile, "--data-binary '@payload.bin'") {
		t.Fatalf("curl file:\n%s", curlFile)
	}
	if !strings.HasSuffix(xhFile, "< 'payload.bin'") {
		t.Fatalf("xh file:\n%s", xhFile)
	}
	if !strings.Contains(xhMulti, "--multipart") || !strings.Contains(xhMulti, "'f@a.png;type=image/png'") {
		t.Fatalf("xh multipart:\n%s", xhMulti)
	}
	if !strings.Contains(psMulti, "-Form @{") || !strings.Contains(psMulti, "Get-Item -Path 'a.png'") {
		t.Fatalf("powershell multipart:\n%s", psMulti)
	}
	if !strings.HasPrefix(wgetMulti, "# wget cannot send multipart/form-data") {
		t.Fatalf("wget multipart:\n%s", wgetMulti)
	}
}

func TestGenerate_should_keep_unresolved_names_verbatim(t *testing.T) {
	r := engine.PreviewRequest{Method: "GET", URL: "{{env.base_url}}/{{$no-such-fn}}"}

	got, err := Generate(Curl, r)

	if err != nil || got != "curl '{{env.base_url}}/{{$no-such-fn}}'" {
		t.Fatalf("err %v got %q", err, got)
	}
}

func TestGenerate_should_refuse_an_unknown_format(t *testing.T) {
	_, err := Generate(Format("fortran"), engine.PreviewRequest{})

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestFormats_should_list_every_format_with_a_label(t *testing.T) {
	got := Formats()

	if len(got) != 7 || got[0] != Curl || Label(PowerShell) == "" || Label(Xh) == "" || Label(HTTPFile) == "" || Label(Fetch) == "" || Label(Python) == "" {
		t.Fatalf("formats %v", got)
	}
}

func TestGenerate_should_render_a_json_post_as_a_http_file_fetch_and_python(t *testing.T) {
	// Before: only four shell tools were offered.
	tests := []struct {
		format Format
		want   string
	}{
		{HTTPFile, "POST https://api.example.com/v1/users?x=1\nContent-Type: application/json\nAuthorization: Bearer it's\n\n{\"name\":\"Ada\"}"},
		{Fetch, "const response = await fetch(\"https://api.example.com/v1/users?x=1\", {\n  method: \"POST\",\n  headers: {\n    \"Content-Type\": \"application/json\",\n    \"Authorization\": \"Bearer it's\",\n  },\n  body: \"{\\\"name\\\":\\\"Ada\\\"}\",\n})"},
		{Python, "import requests\n\nresponse = requests.request(\n    \"POST\",\n    \"https://api.example.com/v1/users?x=1\",\n    headers={\n        \"Content-Type\": \"application/json\",\n        \"Authorization\": \"Bearer it's\",\n    },\n    data=\"{\\\"name\\\":\\\"Ada\\\"}\".encode(),\n)"},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got, err := Generate(tt.format, jsonPost())

			if err != nil || got != tt.want {
				t.Fatalf("err %v\n--- got\n%s\n--- want\n%s", err, got, tt.want)
			}
		})
	}
}

func TestGenerate_Given_a_plain_get_Then_the_code_formats_have_no_headers_or_body(t *testing.T) {
	r := engine.PreviewRequest{URL: "https://x.test/?a=1&b=<2>"}

	http, _ := Generate(HTTPFile, r)
	fetch, _ := Generate(Fetch, r)
	py, _ := Generate(Python, r)

	if http != "GET https://x.test/?a=1&b=<2>" {
		t.Errorf("http %q", http)
	}
	if fetch != "const response = await fetch(\"https://x.test/?a=1&b=<2>\", {\n  method: \"GET\",\n})" {
		t.Errorf("fetch %q", fetch)
	}
	if py != "import requests\n\nresponse = requests.request(\n    \"GET\",\n    \"https://x.test/?a=1&b=<2>\",\n)" {
		t.Errorf("python %q", py)
	}
}

func TestGenerate_Given_a_multipart_body_Then_each_code_format_sends_the_parts(t *testing.T) {
	r := engine.PreviewRequest{
		Method: "POST", URL: "https://x.test/up",
		Headers:  []engine.Header{{Name: "Content-Type", Value: "multipart/form-data"}},
		BodyKind: "multipart",
		Parts:    []engine.PreviewPart{{Name: "note", Value: "hi"}, {Name: "doc", File: "./a.pdf", ContentType: "application/pdf"}},
	}

	http, _ := Generate(HTTPFile, r)
	fetch, _ := Generate(Fetch, r)
	py, _ := Generate(Python, r)

	for name, want := range map[string][]string{
		http:  {"boundary=flowi-boundary", "name=\"note\"\n\nhi", "filename=\"./a.pdf\"", "Content-Type: application/pdf", "< ./a.pdf", "--flowi-boundary--"},
		fetch: {"const form = new FormData()", "form.append(\"note\", \"hi\")", "openAsBlob(\"./a.pdf\")", "body: form,"},
		py:    {"files={", "\"note\": (None, \"hi\")", "\"doc\": (\"./a.pdf\", open(\"./a.pdf\", \"rb\"), \"application/pdf\")"},
	} {
		for _, w := range want {
			if !strings.Contains(name, w) {
				t.Errorf("missing %q in\n%s", w, name)
			}
		}
	}
	if strings.Contains(py, "headers=") {
		t.Errorf("requests writes the multipart Content-Type itself, but the snippet sets one:\n%s", py)
	}
}
