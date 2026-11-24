// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/snippet"
	"github.com/githugt/flowi/lib/vault"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func newSnippetVM(t *testing.T, b *vmtest.Backend, exclude bool) (*vm.Snippet, *vm.Request) {
	t.Helper()
	req, w := newRequestVM(t, b)
	ws := vm.NewWorkspace(b, w)
	return vm.NewSnippet(b, w, ws, req, exclude), req
}

func TestSnippetVM_should_preview_the_edited_version_and_render_curl_by_default(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "POST", URL: "https://s/login"}}
	s, req := newSnippetVM(t, b, true)
	req.Edit(func(v *model.RequestVersion) { v.URL = "https://edited" })

	s.Refresh()

	if s.Text() != "curl -X POST 'https://s/login'" || s.Err() != "" {
		t.Fatalf("text %q err %q", s.Text(), s.Err())
	}
	if len(b.PreviewVersions) != 1 || b.PreviewVersions[0].URL != "https://edited" {
		t.Fatalf("previewed %+v", b.PreviewVersions)
	}
}

// Before: only an HTTP request had a Code section, and a gRPC call would have been written as a curl command.
func TestSnippetVM_Given_a_grpc_message_or_query_Then_the_format_is_the_tool_of_its_kind_and_secrets_stay_out(t *testing.T) {
	tests := []struct {
		name    string
		preview engine.PreviewRequest
		format  snippet.Format
		want    string
	}{
		{"grpc", engine.PreviewRequest{Method: "GRPC", URL: "grpc://h:50051/p.S/M", BodyKind: "text", Body: "{}"}, snippet.Grpcurl, "grpcurl"},
		{"mqtt", engine.PreviewRequest{Method: "MQTT", URL: "mqtt://u:" + vault.Mask + "@h/t", BodyKind: "text", Body: "x"}, snippet.Mosquitto, "-P '<secret>'"},
		{"sql", engine.PreviewRequest{Method: "SQL", URL: "postgres://u:" + vault.Mask + "@h/db", BodyKind: "text", Body: "SELECT 1"}, snippet.Psql, "psql 'postgres://u:<secret>@h/db'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := sampleBackend()
			b.PreviewResult = engine.Preview{Masked: tt.preview}
			s, _ := newSnippetVM(t, b, true)

			s.Refresh()

			if s.Format() != tt.format || !strings.Contains(s.Text(), tt.want) || strings.Contains(s.Text(), vault.Mask) || s.Err() != "" {
				t.Fatalf("format %q text %q err %q", s.Format(), s.Text(), s.Err())
			}
			if f := s.Formats(); len(f) != 1 || f[0] != tt.format {
				t.Fatalf("formats %v", f)
			}
		})
	}
}

func TestSnippetVM_Given_a_kind_no_common_tool_speaks_Then_it_says_so_instead_of_writing_curl(t *testing.T) {
	// Before: SQS, SNS, Pub/Sub, Service Bus, AMQP, SSE and data files said this; now only AMQP 1.0 and a folder of data
	// files do, as no common command-line tool speaks them.
	b := sampleBackend()
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "AMQP1", URL: "amqp://queue", BodyKind: "text", Body: "x"}}
	s, _ := newSnippetVM(t, b, true)

	s.Refresh()

	if s.Text() != "" || !strings.Contains(s.Err(), "No common command-line tool") || len(s.Formats()) != 0 {
		t.Fatalf("text %q err %q", s.Text(), s.Err())
	}
}

func TestSnippetVM_Given_exclude_secrets_Then_masked_values_become_placeholders(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{
		Masked: engine.PreviewRequest{Method: "GET", URL: "https://s/", Headers: []engine.Header{{Name: "Authorization", Value: "Bearer " + vault.Mask}}},
		Real:   &engine.PreviewRequest{Method: "GET", URL: "https://s/", Headers: []engine.Header{{Name: "Authorization", Value: "Bearer hunter2"}}},
	}
	s, _ := newSnippetVM(t, b, true)

	s.Refresh()

	if strings.Contains(s.Text(), "hunter2") || strings.Contains(s.Text(), vault.Mask) || !strings.Contains(s.Text(), "<secret>") {
		t.Fatalf("text %q", s.Text())
	}
	if len(b.PreviewOpts) != 1 {
		t.Fatalf("calls %d", len(b.PreviewOpts))
	}
}

func TestSnippetVM_Given_include_secrets_Then_real_values_are_shown(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{
		Masked: engine.PreviewRequest{Method: "GET", URL: "https://s/", Headers: []engine.Header{{Name: "Authorization", Value: "Bearer " + vault.Mask}}},
		Real:   &engine.PreviewRequest{Method: "GET", URL: "https://s/", Headers: []engine.Header{{Name: "Authorization", Value: "Bearer hunter2"}}},
	}
	s, _ := newSnippetVM(t, b, true)

	s.SetExclude(false)

	if !strings.Contains(s.Text(), "hunter2") || s.Exclude() {
		t.Fatalf("text %q exclude %v", s.Text(), s.Exclude())
	}
}

func TestSnippetVM_should_switch_formats_without_asking_the_backend_again(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "GET", URL: "https://s/"}}
	s, _ := newSnippetVM(t, b, true)
	s.Refresh()

	s.SetFormat(snippet.Xh)

	if s.Text() != "xh 'https://s/'" || len(b.PreviewOpts) != 1 {
		t.Fatalf("text %q calls %d", s.Text(), len(b.PreviewOpts))
	}
}

func TestSnippetVM_Given_a_preview_problem_Then_shows_it(t *testing.T) {
	b := sampleBackend()
	b.PreviewRequestErr = errBoom
	s, _ := newSnippetVM(t, b, true)

	s.Refresh()

	if s.Err() == "" || s.Text() != "" {
		t.Fatalf("err %q text %q", s.Err(), s.Text())
	}
}

// Review finding 8: the snippet kept the old environment's values.
func TestSnippetVM_should_refresh_when_the_environment_changes(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "GET", URL: "https://s/"}}
	req, w := newRequestVM(t, b)
	ws := vm.NewWorkspace(b, w)
	s := vm.NewSnippet(b, w, ws, req, true)
	s.Refresh()
	before := len(b.PreviewOpts)

	ws.SelectEnv("prod")
	ws.SelectEnv("prod") // unchanged: no new preview

	if len(b.PreviewOpts) != before+1 || b.PreviewOpts[before].Env != "prod" {
		t.Fatalf("previews %d, last env %q", len(b.PreviewOpts), b.PreviewOpts[len(b.PreviewOpts)-1].Env)
	}
}

// Review finding 5: with secrets included, a cancelled passphrase prompt
// came back with every keystroke and also blanked the Request tab.
func TestSnippetVM_Given_the_real_values_fail_Then_falls_back_to_excluding_and_keeps_the_masked_preview(t *testing.T) {
	b := sampleBackend()
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "GET", URL: "https://s/"}}
	b.PreviewRealErr = errBoom
	s, _ := newSnippetVM(t, b, false)

	s.Refresh()
	exclude, request, text, problem := s.Exclude(), s.Request(), s.Text(), s.Err()
	callsAfterFirst := len(b.PreviewOpts)
	s.Refresh()

	if !exclude || request.URL != "https://s/" || text == "" || !strings.Contains(problem, "boom") {
		t.Fatalf("exclude %v request %+v text %q err %q", exclude, request, text, problem)
	}
	if len(b.PreviewOpts) != callsAfterFirst+1 {
		t.Fatalf("a second refresh asked for the real values again: %d calls after %d", len(b.PreviewOpts), callsAfterFirst)
	}
}

func TestSnippetVM_Close_should_stop_refreshing(t *testing.T) {
	b := sampleBackend()
	s, _ := newSnippetVM(t, b, true)
	s.Close()

	s.Refresh()

	if len(b.PreviewOpts) != 0 {
		t.Fatalf("a closed snippet previewed %d times", len(b.PreviewOpts))
	}
}
