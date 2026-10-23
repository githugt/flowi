// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// AuthView is the Authorization header as the Auth section shows it.
type AuthView struct {
	Kind  string `json:"kind"` // none, bearer, basic or other
	Value string `json:"value"`
}

func authKinds() map[string]vm.AuthKind {
	return map[string]vm.AuthKind{"none": vm.AuthNone, "bearer": vm.AuthBearer, "basic": vm.AuthBasic, "other": vm.AuthOther}
}

func authViewOf(r *vm.Request) AuthView {
	kind, value := r.Auth()
	for name, k := range authKinds() {
		if k == kind {
			return AuthView{Kind: name, Value: value}
		}
	}
	return AuthView{Kind: "none"}
}

// SetMethod sets the HTTP verb.
func (s *RequestService) SetMethod(node, method string) {
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) { v.Method = method })
	})
}

// SetURL sets the URL; a query string typed into it moves to the query rows.
func (s *RequestService) SetURL(node, url string) {
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) { v.URL, v.Query = vm.SplitURL(url, v.Query) })
	})
}

// SetRows replaces the rows of a table: query, headers or variables.
func (s *RequestService) SetRows(node, table string, rows []model.KeyValue) error {
	var err error
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) {
			switch table {
			case "query":
				v.Query = rows
			case "headers":
				if a := v.Async; a != nil && !vm.AsyncHeadersAreConnection(a.Protocol) {
					if a.Send == nil {
						a.Send = &model.AsyncSend{}
					}
					a.Send.Headers = rows // a message carries its own headers; only ws and sse have connection headers
					return
				}
				v.Headers = rows
			case "variables":
				v.Variables = rows
			default:
				err = fmt.Errorf("unknown table %q", table)
			}
		})
	})
	return err
}

// SetBodyKind chooses none, text, form or file.
func (s *RequestService) SetBodyKind(node, kind string) {
	s.with(node, func(o *openRequest) { o.req.SetBodyKind(vm.BodyKind(kind)) })
}

// SetBodyFormat chooses json, xml, yaml or raw for a text body.
func (s *RequestService) SetBodyFormat(node, format string) {
	s.with(node, func(o *openRequest) { o.req.SetBodyFormat(vm.BodyFormat(format)) })
}

// SetMultipart sends a form as multipart/form-data (on) or urlencoded (off).
func (s *RequestService) SetMultipart(node string, on bool) {
	s.with(node, func(o *openRequest) { o.req.SetMultipart(on) })
}

// SetBodyText sets the text of a text body, in the chosen format's field.
func (s *RequestService) SetBodyText(node, text string) {
	s.with(node, func(o *openRequest) {
		json := o.req.BodyFormat() == vm.FormatJSON
		o.req.Edit(func(v *model.RequestVersion) {
			v.Body.JSON, v.Body.Text = "", ""
			if json {
				v.Body.JSON = text
			} else {
				v.Body.Text = text
			}
		})
	})
}

// SetFormRows sets the rows of a urlencoded form body.
func (s *RequestService) SetFormRows(node string, rows []model.KeyValue) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Body.Form = rows }) })
}

// SetBodyFile sets the file sent as the body.
func (s *RequestService) SetBodyFile(node, path string) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Body.File = path }) })
}

// SetAuth sets the Authorization header: kind is none, bearer, basic or other.
func (s *RequestService) SetAuth(node, kind, value string) {
	s.with(node, func(o *openRequest) { o.req.SetAuth(authKinds()[kind], value) })
}

// SetExtract replaces the values taken from the response.
func (s *RequestService) SetExtract(node string, rows []model.Extraction) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Extract = rows }) })
}

// SetAssert replaces the assertions on the response.
func (s *RequestService) SetAssert(node string, lines []string) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Assert = lines }) })
}

// SetVersionDetails sets the title and the description (Markdown, the version's
// docs) of the selected version.
func (s *RequestService) SetVersionDetails(node, name, docs string) {
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) { v.Name, v.Docs = strings.TrimSpace(name), docs })
	})
}

// SetDocs sets the notes of the selected version.
func (s *RequestService) SetDocs(node, docs string) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Docs = docs }) })
}
