// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/importers/curl"
	"github.com/githugt/flowi/lib/importers/httpfile"
	"github.com/githugt/flowi/lib/model"
)

// Text pasted into a request's address (a curl command, a .http request, an
// API description) is offered to the user: the window recognises the kind,
// then fills the open request from it or previews importing it.

// Errors FillFromPaste and PreviewPasted return.
var (
	ErrPasteNotFillable = errors.New("only a curl command or a .http request can fill a request")
	ErrPasteNoRequest   = errors.New("the pasted text has no request")
	ErrPasteNotHTTP     = errors.New("the pasted request is an HTTP request; import it as a new request instead")
	ErrPasteKind        = errors.New("pasted text of this kind cannot be imported")
)

// pasteFillers parse the kinds whose first request can fill an open request.
func pasteFillers() map[string]func([]byte, string) (importers.Result, diag.List) {
	return map[string]func([]byte, string) (importers.Result, diag.List){
		string(flowi.ImportCurl): curl.Import,
		string(flowi.ImportHTTP): httpfile.Import,
	}
}

// pastedKinds are the kinds a paste may import: those that make requests.
// Bruno is a folder, and Mermaid sketches a flow, not a request.
func pastedKinds() []flowi.ImportKind {
	return []flowi.ImportKind{flowi.ImportCurl, flowi.ImportHTTP, flowi.ImportOpenAPI, flowi.ImportAsyncAPI, flowi.ImportPostman, flowi.ImportHAR}
}

// FillFromPaste replaces the open HTTP request's method, URL, query, headers
// and body with those of the first request in text, a curl command or a .http
// request (kind curl or http). Its name, checks and the rest stay. It returns
// how many requests text holds, so the window can say the others were left.
func (s *RequestService) FillFromPaste(node, kind, text string) (int, error) {
	if kind == pasteKindRequestFile {
		return s.fillFromRequestFile(node, text)
	}
	from, count, err := firstPastedRequest(kind, text)
	if err != nil {
		return 0, err
	}
	s.with(node, func(o *openRequest) {
		cur := o.req.Current()
		if cur.Family() != model.FamilyHTTP {
			err = ErrPasteNotHTTP
			return
		}
		o.req.Edit(func(v *model.RequestVersion) {
			v.Method, v.URL, v.Query, v.Headers, v.Body = from.Method, from.URL, from.Query, from.Headers, from.Body
		})
	})
	return count, err
}

// PastedSecrets are the secret variables a fill from text of the given kind would reference in place of a
// credential in the text ("{{secret.authorization}}"), without the credential itself: the window says so before
// the fill and points to Secrets for the value. Nil for text that holds none, or that cannot fill.
// Before: the credential was dropped and the request got a reference to a secret that had no value, with only a
// hint about a command line tool in the diagnostics that the window did not show.
func (s *RequestService) PastedSecrets(kind, text string) []string {
	parse, ok := pasteFillers()[kind]
	if !ok {
		return nil
	}
	res, _ := parse([]byte(text), "pasted text")
	var out []string
	for _, it := range res.Items {
		for _, d := range it.Diags {
			if p := d.Args["placeholder"]; d.Code == diag.CodeImportSensitive && p != "" && !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// pasteKindRequestFile is the kind of text that is a Flowi request file (the YAML of a `*.req.yaml`).
const pasteKindRequestFile = "flowi"

// fillFromRequestFile replaces the open request's version with the default version of a pasted request
// file: any kind (HTTP, message or gRPC), so a message request can be filled from the file of another.
// The version's label and the request's name, id and other versions stay.
func (s *RequestService) fillFromRequestFile(node, text string) (int, error) {
	req, diags := flowi.ParseRequest([]byte(text), "pasted request")
	if diags.HasErrors() {
		return 0, fmt.Errorf("reading the pasted request: %w", diags.Err())
	}
	if len(req.Versions) == 0 {
		return 0, ErrPasteNoRequest
	}
	from := defaultVersion(req)
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) {
			label := v.Label
			*v = from
			v.Label = label
		})
	})
	return 1, nil
}

// firstPastedRequest parses text and returns its first request's default
// version, and how many requests it holds.
func firstPastedRequest(kind, text string) (model.RequestVersion, int, error) {
	parse, ok := pasteFillers()[kind]
	if !ok {
		return model.RequestVersion{}, 0, fmt.Errorf("filling from %q: %w", kind, ErrPasteNotFillable)
	}
	res, diags := parse([]byte(text), "pasted text")
	if diags.HasErrors() {
		return model.RequestVersion{}, 0, fmt.Errorf("reading the pasted %s: %w", kind, diags.Err())
	}
	if len(res.Requests) == 0 {
		// A command the importer skipped says why in its item.
		if err := res.Diags().Err(); err != nil {
			return model.RequestVersion{}, 0, fmt.Errorf("reading the pasted %s: %w", kind, err)
		}
		return model.RequestVersion{}, 0, ErrPasteNoRequest
	}
	return defaultVersion(res.Requests[0]), len(res.Requests), nil
}

// defaultVersion is the version a request sends by default: the one Default
// names, else its first.
func defaultVersion(r model.Request) model.RequestVersion {
	if i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == r.Default }); i >= 0 {
		return r.Versions[i]
	}
	if len(r.Versions) == 0 {
		return model.RequestVersion{}
	}
	return r.Versions[0]
}

// PreparePasted sets the import wizard to import text of the given kind into
// the collection of the tree row node, without previewing: the import screen
// opens with the text as its source.
//
// Before: a kind the wizard reads from a file (an API description, a Postman export) got the text written to a temporary
// file in the system's temp folder, kept until the window closed: the document sat on disk, and its kept copy in the
// collection was named after the file ("flowi-paste-2749985858"). Every kind now takes the text as it is.
func (s *ImportService) PreparePasted(kind, text, node string) error {
	k := flowi.ImportKind(kind)
	if !slices.Contains(pastedKinds(), k) {
		return fmt.Errorf("importing %q: %w", kind, ErrPasteKind)
	}
	s.app.actor.Do(func() {
		s.vm.SetKind(k)
		s.vm.SetFile("")
		s.vm.SetPaste(text)
		if c := s.app.workspace.Node(node).Collection; c != "" {
			s.vm.SetCollection(c)
		}
	})
	return nil
}

// PreviewPasted is PreparePasted, then previews; the plan arrives as the
// "import" event.
func (s *ImportService) PreviewPasted(kind, text, node string) error {
	if err := s.PreparePasted(kind, text, node); err != nil {
		return err
	}
	s.app.actor.Do(s.vm.Preview)
	return nil
}

// stalePasteAge is how old a pasted-text file must be before a launch removes it: a file of a window that is still
// open is much younger.
const stalePasteAge = 12 * time.Hour

// sweepPasted removes the pasted-text files an earlier version left in dir (it wrote pasted documents to the
// temp folder), and returns how many it removed. Only files this service names and that are older than olderThan go.
//
// Before: a pasted collection or HAR stayed in the temp folder until the app quit cleanly, and after a crash
// it stayed for good.
func sweepPasted(dir string, olderThan time.Duration, now time.Time) int {
	files, err := filepath.Glob(filepath.Join(dir, "flowi-paste-*"))
	if err != nil {
		return 0
	}
	removed := 0
	for _, f := range files {
		info, err := os.Lstat(f)
		if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) < olderThan {
			continue
		}
		if os.Remove(f) == nil {
			removed++
		}
	}
	return removed
}
