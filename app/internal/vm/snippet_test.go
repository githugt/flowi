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
