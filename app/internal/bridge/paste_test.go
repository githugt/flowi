// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"os"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestRequestService_FillFromPaste_should_take_the_first_pasted_request(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		text       string
		wantMethod string
		wantURL    string
		wantHeader string
		wantCount  int
	}{
		{
			name: "a curl command", kind: "curl", text: "curl --location 'https://pokeapi.co/api/v2/pokemon/ditto' -H 'Accept: application/json'",
			wantMethod: "GET", wantURL: "https://pokeapi.co/api/v2/pokemon/ditto", wantHeader: "Accept", wantCount: 1,
		},
		{
			name: "two curl commands", kind: "curl", text: "curl https://example.com/a\ncurl -X DELETE https://example.com/b",
			wantMethod: "GET", wantURL: "https://example.com/a", wantCount: 2,
		},
		{
			name: "a .http request", kind: "http", text: "POST https://example.com/orders\nContent-Type: application/json\n\n{\"id\": 1}\n",
			wantMethod: "POST", wantURL: "https://example.com/orders", wantHeader: "Content-Type", wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, v, _, _ := openLogin(t)

			count, err := reqs.FillFromPaste(v.Node, tt.kind, tt.text)
			got, _ := reqs.Open(v.Node)

			if err != nil || count != tt.wantCount {
				t.Fatalf("got (%d, %v), want (%d, nil)", count, err, tt.wantCount)
			}
			if got.Current.Method != tt.wantMethod || got.Current.URL != tt.wantURL || got.Name != "Login" || !got.Dirty {
				t.Fatalf("got %s %s (name %q, dirty %v), want %s %s, the name kept, dirty",
					got.Current.Method, got.Current.URL, got.Name, got.Dirty, tt.wantMethod, tt.wantURL)
			}
			if tt.wantHeader != "" && !hasKey(got.Current.Headers, tt.wantHeader) {
				t.Fatalf("headers %+v, want %s", got.Current.Headers, tt.wantHeader)
			}
		})
	}
}

func TestRequestService_FillFromPaste_should_refuse_what_cannot_fill_a_request(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		text    string
		wantErr error // nil: any error
	}{
		{name: "a kind that only imports", kind: "openapi", text: `{"openapi": "3.1.0"}`, wantErr: bridge.ErrPasteNotFillable},
		{name: "a curl without a URL", kind: "curl", text: "curl -X GET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, v, _, _ := openLogin(t)

			_, err := reqs.FillFromPaste(v.Node, tt.kind, tt.text)
			got, _ := reqs.Open(v.Node)

			if err == nil || (tt.wantErr != nil && !errors.Is(err, tt.wantErr)) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
			if got.Dirty {
				t.Fatal("a refused paste changed the request")
			}
		})
	}
}

func TestRequestService_FillFromPaste_Given_an_async_request_Then_it_is_refused(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R8", Name: "Events"}, Path: "events", File: "/c/requests/events.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: "nats", Server: "nats://localhost", Channel: "orders", Send: &model.AsyncSend{}}}}})
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	node := childNamed(t, app, "Events")
	if _, err := app.Requests().Open(node); err != nil {
		t.Fatal(err)
	}

	_, err := app.Requests().FillFromPaste(node, "curl", "curl https://example.com")

	if !errors.Is(err, bridge.ErrPasteNotHTTP) {
		t.Fatalf("got %v, want ErrPasteNotHTTP", err)
	}
}

func TestImportService_PreviewPasted_should_preview_the_text_into_the_rows_collection(t *testing.T) {
	tests := []struct {
		name string
		kind flowi.ImportKind
		text string
	}{
		{name: "a kind the wizard takes pasted", kind: flowi.ImportCurl, text: "curl https://example.com"},
		{name: "a kind the wizard reads from a file", kind: flowi.ImportOpenAPI, text: "openapi: 3.1.0\ninfo: {title: Shop, version: '1'}\npaths: {}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v, _, app, b := openLoginWith(t)
			importer := app.Import()
			t.Cleanup(func() { _ = importer.Close() })

			err := importer.PreviewPasted(string(tt.kind), tt.text, v.Node)

			if err != nil || len(b.PreviewedSources) != 1 {
				t.Fatalf("got %v and %d previews, want one preview", err, len(b.PreviewedSources))
			}
			if src := b.PreviewedSources[0]; src.Kind != tt.kind || string(src.Data) != tt.text {
				t.Fatalf("previewed %s %q, want %s %q", src.Kind, src.Data, tt.kind, tt.text)
			}
			if got := importer.State().Collection; got != "C1" {
				t.Fatalf("collection %q, want the request's C1", got)
			}
		})
	}
}

func TestImportService_PreparePasted_should_fill_the_wizard_with_the_text_and_not_preview(t *testing.T) {
	tests := []struct {
		name string
		kind flowi.ImportKind
		text string
	}{
		{name: "a kind the wizard takes pasted", kind: flowi.ImportCurl, text: "curl https://example.com"},
		{name: "a kind the wizard reads from a file", kind: flowi.ImportOpenAPI, text: "openapi: 3.1.0\ninfo: {title: Shop, version: '1'}\npaths: {}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v, _, app, b := openLoginWith(t)
			importer := app.Import()
			t.Cleanup(func() { _ = importer.Close() })

			err := importer.PreparePasted(string(tt.kind), tt.text, v.Node)

			got := importer.State()
			if err != nil || len(b.PreviewedSources) != 0 {
				t.Fatalf("got %v and %d previews, want none yet", err, len(b.PreviewedSources))
			}
			if got.Kind != string(tt.kind) || !got.Pasted || got.Collection != "C1" {
				t.Fatalf("kind %q pasted %v collection %q", got.Kind, got.Pasted, got.Collection)
			}
		})
	}
}

func TestImportService_State_Given_a_chosen_file_Then_it_is_not_marked_pasted(t *testing.T) {
	_, _, _, app, _ := openLoginWith(t)
	importer := app.Import()
	importer.SetFile("/some/spec.yaml")

	got := importer.State()

	if got.Pasted {
		t.Fatalf("a file the person chose reads as pasted text")
	}
}

func TestImportService_PreviewPasted_should_refuse_kinds_that_make_no_requests(t *testing.T) {
	_, v, _, app, b := openLoginWith(t)

	err := app.Import().PreviewPasted(string(flowi.ImportMermaid), "flowchart LR\n  a --> b", v.Node)

	if !errors.Is(err, bridge.ErrPasteKind) || len(b.PreviewedSources) != 0 {
		t.Fatalf("got %v and %d previews, want ErrPasteKind and none", err, len(b.PreviewedSources))
	}
}

func TestImportService_Close_should_remove_the_pasted_text(t *testing.T) {
	_, v, _, app, _ := openLoginWith(t)
	importer := app.Import()
	if err := importer.PreviewPasted(string(flowi.ImportOpenAPI), "openapi: 3.1.0", v.Node); err != nil {
		t.Fatal(err)
	}
	file := importer.State().File

	err := importer.Close()

	if _, statErr := os.Stat(file); err != nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("close %v, the file %s: %v; want it gone", err, file, statErr)
	}
}

func hasKey(rows []model.KeyValue, key string) bool {
	for _, r := range rows {
		if r.Key == key {
			return true
		}
	}
	return false
}

// childNamed is the id of the tree row labelled label under the first
// collection's requests.
func childNamed(t *testing.T, app *bridge.App, label string) string {
	t.Helper()
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == label {
			return n.ID
		}
	}
	t.Fatalf("no request %q", label)
	return ""
}

const pastedFlowiRequest = `id: 01M63D04BXES8ME7EM4FSWYBEJ
name: Listen
versions:
  "1":
    async:
      protocol: amqp
      server: "amqp://127.0.0.1:5672"
      channel: "flowi.test"
      receive:
        count: 1
        timeout: "30s"
      binding:
        username: "flowi"
        password: "{{secret.broker_password}}"
        queue: "flowi.test"
`

func TestRequestService_FillFromPaste_Given_a_request_file_Then_the_open_request_takes_its_version_and_keeps_its_name(t *testing.T) {
	// Arrange
	reqs, v, _, _ := openLogin(t)

	// Act
	count, err := reqs.FillFromPaste(v.Node, "flowi", pastedFlowiRequest)
	got, _ := reqs.Open(v.Node)

	// Assert
	if err != nil || count != 1 {
		t.Fatalf("got (%d, %v)", count, err)
	}
	a := got.Current.Async
	if a == nil || a.Protocol != "amqp" || a.Server != "amqp://127.0.0.1:5672" || a.Channel != "flowi.test" || len(a.Binding) != 3 {
		t.Fatalf("async %+v", a)
	}
	if got.Name != "Login" || got.Current.Method != "" || got.Current.URL != "" || !got.Dirty {
		t.Fatalf("name %q method %q url %q dirty %v: want the name kept, no HTTP left over, dirty", got.Name, got.Current.Method, got.Current.URL, got.Dirty)
	}
}

func TestRequestService_FillFromPaste_Given_text_that_is_not_a_request_file_Then_nothing_changes(t *testing.T) {
	// Arrange
	reqs, v, _, _ := openLogin(t)

	// Act
	_, err := reqs.FillFromPaste(v.Node, "flowi", "just: some\nother: yaml\n")
	got, _ := reqs.Open(v.Node)

	// Assert
	if err == nil || got.Dirty {
		t.Fatalf("err %v dirty %v", err, got.Dirty)
	}
}
