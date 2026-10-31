// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// AuthView is the Authorization header as the Auth section shows it.
type AuthView struct {
	Kind  string `json:"kind"` // none, bearer, basic or other; "scheme:<type>" when the version has an auth block
	Value string `json:"value"`
	// Fields are the parameters of the auth block's scheme, in the order a form shows them.
	Fields []AuthFieldView `json:"fields"`
	// Schemes are the schemes an auth block can have.
	Schemes []Choice `json:"schemes"`
}

// AuthFieldView is one parameter of an auth block's scheme.
type AuthFieldView struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"` // a credential: written as {{secret.name}}, not typed into the file
	Known  bool   `json:"known"`  // the scheme has this parameter (an unknown one is a typo)
}

func authSchemeChoices() []Choice {
	labels := map[string]string{
		model.AuthAPIKey: "API key", model.AuthDigest: "Digest", model.AuthOAuth2: "OAuth 2.0", model.AuthOAuth1: "OAuth 1.0a",
		model.AuthAWSSigV4: "AWS Signature v4", model.AuthJWT: "JWT bearer", model.AuthHawk: "Hawk", model.AuthMutualTLS: "Client certificate (mTLS)", model.AuthNTLM: "Windows (NTLM)",
		model.AuthHTTPSig: "HTTP message signature (RFC 9421)", model.AuthDPoP: "DPoP proof (RFC 9449)",
	}
	out := make([]Choice, 0, len(labels))
	for _, t := range model.AuthTypes() {
		out = append(out, Choice{ID: "scheme:" + t, Label: labels[t]})
	}
	return out
}

func authKinds() map[string]vm.AuthKind {
	return map[string]vm.AuthKind{"none": vm.AuthNone, "bearer": vm.AuthBearer, "basic": vm.AuthBasic, "other": vm.AuthOther}
}

func authViewOf(r *vm.Request) AuthView {
	if scheme, params := r.AuthScheme(); scheme != "" {
		return AuthView{Kind: "scheme:" + scheme, Fields: authFields(scheme, params), Schemes: authSchemeChoices()}
	}
	kind, value := r.Auth()
	for name, k := range authKinds() {
		if k == kind {
			return AuthView{Kind: name, Value: value, Fields: []AuthFieldView{}, Schemes: authSchemeChoices()}
		}
	}
	return AuthView{Kind: "none", Fields: []AuthFieldView{}, Schemes: authSchemeChoices()}
}

// authFields lists a scheme's parameters with their values, then any other
// parameter the block has.
func authFields(scheme string, params []model.KeyValue) []AuthFieldView {
	secret := model.AuthSecretParams(scheme)
	out := []AuthFieldView{}
	have := map[string]string{}
	for _, kv := range params {
		have[kv.Key] = kv.Value
	}
	for _, name := range model.AuthParams(scheme) {
		out = append(out, AuthFieldView{Name: name, Value: have[name], Secret: slices.Contains(secret, name), Known: true})
		delete(have, name)
	}
	for _, kv := range params {
		if v, ok := have[kv.Key]; ok {
			out = append(out, AuthFieldView{Name: kv.Key, Value: v})
		}
	}
	return out
}

// SetMethod sets the HTTP verb: any method name is accepted, so QUERY and the ones nobody has registered work. A known
// method is stored in its registered spelling ("query" becomes QUERY); any other name is a case-sensitive token and is
// kept as typed, so one that could not go on the wire is refused here rather than at the first send.
// A WebDAV or CalDAV method also fills in the headers and example body it usually needs (vm.ApplyMethodPreset); what
// the request already has stays as it is.
func (s *RequestService) SetMethod(node, method string) error {
	method = strings.TrimSpace(method)
	if !model.ValidMethod(method) {
		return fmt.Errorf("%q is not a method name: use letters, digits and ! # $ %% & ' * + - . ^ _ ` | ~ only, no spaces", method)
	}
	if spelling, known := model.KnownMethod(method); known {
		method = spelling
	}
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(v *model.RequestVersion) {
			v.Method = method
			vm.ApplyMethodPreset(v)
		})
	})
	return nil
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

// SetJSONRPC changes the method or the id of the JSON-RPC call the JSON body holds.
func (s *RequestService) SetJSONRPC(node, field, value string) {
	s.with(node, func(o *openRequest) { o.req.SetJSONRPCField(field, value) })
}

// MakeJSONRPC turns the JSON body into a JSON-RPC call (an existing JSON body becomes its parameters).
func (s *RequestService) MakeJSONRPC(node string) {
	s.with(node, func(o *openRequest) { o.req.MakeJSONRPC() })
}

// SetParts replaces the parts of a multipart body.
func (s *RequestService) SetParts(node string, parts []MultipartPartView) {
	s.with(node, func(o *openRequest) {
		rows := make([]vm.MultipartPart, 0, len(parts))
		for _, p := range parts {
			rows = append(rows, vm.MultipartPart{Name: p.Name, File: p.File, Value: p.Value, ContentType: p.ContentType})
		}
		o.req.SetParts(rows)
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

// SetAuthScheme chooses the scheme of the version's auth block ("" removes the
// block): apikey, digest, oauth2, oauth1, aws_sigv4, jwt, hawk, mtls, ntlm, httpsig or dpop.
func (s *RequestService) SetAuthScheme(node, scheme string) {
	s.with(node, func(o *openRequest) { o.req.SetAuthScheme(scheme) })
}

// SetAuthParam sets one parameter of the auth block; "" removes it.
func (s *RequestService) SetAuthParam(node, name, value string) {
	s.with(node, func(o *openRequest) { o.req.SetAuthParam(name, value) })
}

// SetExtract replaces the values taken from the response.
func (s *RequestService) SetExtract(node string, rows []model.Extraction) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Extract = rows }) })
}

// SetOutcomes replaces the named ways the version can end, in order. An empty list (or nil) is the defaults,
// success and failure.
func (s *RequestService) SetOutcomes(node string, outcomes []model.Outcome) {
	s.with(node, func(o *openRequest) { o.req.Edit(func(v *model.RequestVersion) { v.Outcomes = outcomes }) })
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
