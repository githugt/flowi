// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/card"
)

// CardsOf is the card of every step of f, nested ones included, by node id.
func CardsOf(c model.Collection, f model.Flow) map[string]card.Card {
	out := map[string]card.Card{}
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for _, n := range list {
			out[n.ID] = card.Summarize(n, CardFacts(c, n))
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return out
}

// CardFacts reads what a step's card shows from outside the step: for a
// request step the version it will send, for a flow step the flow it runs. A
// request or flow that cannot be found still gets a card, with its path.
func CardFacts(c model.Collection, n model.Node) card.Facts {
	switch {
	case n.Request != nil:
		return requestFacts(c, n)
	case n.Subflow != nil:
		return flowFacts(c, n.Subflow.Flow)
	}
	return card.Facts{}
}

// hooksThatRun is how many of a request's hooks of one phase run for a step:
// none when the step switches hooks off or skips that phase.
func hooksThatRun(control model.HookControl, phase string, defined int) int {
	if control.Mode == "none" || slices.Contains(control.Skip, phase) {
		return 0
	}
	return defined
}

// sameRequest says whether ref points at r: by id, or by path when it has no id.
func sameRequest(r model.Request, ref model.Ref) bool {
	if ref.ID == "" {
		return r.Path == ref.Path
	}
	return r.ID == ref.ID
}

func flowFacts(c model.Collection, ref model.Ref) card.Facts {
	for _, f := range c.Flows {
		if f.ID == ref.ID || (ref.ID == "" && f.Path == ref.Path) {
			return card.Facts{Target: firstText(f.Name, f.Path), Returns: len(f.Outputs)}
		}
	}
	return card.Facts{Target: ref.Path}
}

func requestFacts(c model.Collection, n model.Node) card.Facts {
	f := requestVersionFacts(c, n)
	if len(n.Request.Outcomes) > 0 {
		f.Outcomes = outcomeFactsOf(n.Request.Outcomes) // the step's own list wins over the request's
	}
	return f
}

func requestVersionFacts(c model.Collection, n model.Node) card.Facts {
	ref := n.Request.Request
	for _, r := range c.Requests {
		if !sameRequest(r, ref) {
			continue
		}
		v, ok := versionFor(r, n.Request.Version)
		if !ok {
			return card.Facts{Kind: "http", Target: r.Path}
		}
		f := versionFacts(v)
		f.PreHooks, f.PostHooks = hooksThatRun(n.Request.Hooks, "pre", len(r.Hooks.Pre)+len(v.Hooks.Pre)), hooksThatRun(n.Request.Hooks, "post", len(r.Hooks.Post)+len(v.Hooks.Post))
		f.Retry, f.Timeout = settingInt(v.Settings, "retry.max"), settingText(v.Settings, "timeout")
		f.Outcomes = outcomeFactsOf(v.Outcomes)
		return f
	}
	return card.Facts{Kind: "http", Target: ref.Path}
}

// versionFor is the version a step pins, else the request's default, else its first.
func versionFor(r model.Request, label string) (model.RequestVersion, bool) {
	for _, want := range []string{label, r.Default} {
		for _, v := range r.Versions {
			if want != "" && v.Label == want {
				return v, true
			}
		}
	}
	if len(r.Versions) > 0 {
		return r.Versions[0], true
	}
	return model.RequestVersion{}, false
}

// outcomeFacts are the outcomes the version writes, nil when it writes none or an empty list (success and failure then).
func outcomeFactsOf(list []model.Outcome) []card.OutcomeFact {
	if len(list) == 0 {
		return nil
	}
	out := make([]card.OutcomeFact, 0, len(list))
	for _, o := range list {
		out = append(out, card.OutcomeFact{Name: o.Name, Disabled: o.Disabled, Color: o.Color, Description: o.Description})
	}
	return out
}

func versionFacts(v model.RequestVersion) card.Facts {
	switch {
	case v.Async != nil:
		return asyncFacts(v)
	case v.GRPC != nil:
		return grpcFacts(v)
	case v.Dataset != nil:
		return datasetFacts(v)
	}
	headers, auth := withoutAuthorization(v.Headers)
	if v.Auth != nil {
		auth = v.Auth.Type
	}
	return card.Facts{
		Kind: "http", Method: v.Method, Target: pathOf(v.URL), Auth: auth,
		Query: len(v.Query), Headers: len(headers), Body: bodyKind(v.Body),
	}
}

func grpcFacts(v model.RequestVersion) card.Facts {
	g := v.GRPC
	method := "unary"
	if g.Receive != nil {
		method = "stream"
	}
	_, auth := withoutAuthorization(v.Headers)
	if v.Auth != nil {
		auth = v.Auth.Type
	}
	return card.Facts{Kind: "grpc", Method: method, Target: g.Method, Auth: auth, Body: bodyKind(g.Body), Props: len(v.Headers)}
}

func datasetFacts(v model.RequestVersion) card.Facts {
	d := v.Dataset
	return card.Facts{Kind: "dataset", Method: datasetVerb, Target: datasetTip(*d), Sends: true, Body: d.Driver, Props: len(d.Params)}
}

func asyncFacts(v model.RequestVersion) card.Facts {
	a := v.Async
	f := card.Facts{Kind: a.Protocol, Target: a.Channel, Sends: a.Send != nil, Receives: a.Receive != nil}
	if a.Send != nil {
		f.Body, f.Props = bodyKind(a.Send.Body), len(a.Send.Headers)
		f.SendExtras = sendExtras(a)
	}
	if a.Receive != nil {
		f.ReceiveCount, f.ReceiveWait = a.Receive.Count, durationText(a.Receive.Timeout)
		f.ReceiveExtras = receiveExtras(a)
	}
	_, f.Auth = withoutAuthorization(v.Headers)
	if v.Auth != nil {
		f.Auth = v.Auth.Type
	}
	if a.Send != nil && a.Receive != nil && a.Protocol == "nats" {
		f.Method = "request"
	}
	return f
}

// sendExtras are the protocol options worth a word on the card.
func sendExtras(a *model.Async) []string {
	var out []string
	if q := binding(a, "qos"); q != "" && a.Protocol == "mqtt" {
		out = append(out, "QoS "+q)
	}
	if binding(a, "retain") == "true" {
		out = append(out, "retain")
	}
	if binding(a, "key") != "" {
		out = append(out, "key")
	}
	if s := binding(a, "session"); s != "" {
		out = append(out, "session "+s)
	}
	return out
}

func receiveExtras(a *model.Async) []string {
	var out []string
	if g := binding(a, "group"); g != "" {
		out = append(out, "group "+g)
	}
	if binding(a, "ack") == "auto" {
		out = append(out, "auto-ack")
	}
	return out
}

func binding(a *model.Async, name string) string {
	for _, kv := range a.Binding {
		if kv.Key == name {
			return kv.Value
		}
	}
	return ""
}

// withoutAuthorization drops the Authorization header from headers and says
// what scheme it carried ("Bearer", "Basic"); it is shown as auth, not as a header.
func withoutAuthorization(headers []model.KeyValue) (rest []model.KeyValue, scheme string) {
	for _, h := range headers {
		if !strings.EqualFold(h.Key, "Authorization") {
			rest = append(rest, h)
			continue
		}
		scheme, _, _ = strings.Cut(strings.TrimSpace(h.Value), " ")
	}
	return rest, scheme
}

func bodyKind(b model.Body) string {
	switch {
	case b.JSON != "":
		return "JSON"
	case b.Text != "":
		return "text"
	case len(b.Form) > 0:
		return "form"
	case len(b.Multipart) > 0:
		return "multipart"
	case b.File != "":
		return "file"
	}
	return ""
}

// pathOf is a request URL without its scheme and host: the path a person
// recognises. A leading template ({{env.base_url}}) stands for the host.
func pathOf(url string) string {
	url, _, _ = strings.Cut(url, "?")
	if i := strings.Index(url, "}}"); i >= 0 && strings.HasPrefix(url, "{{") {
		return url[i+2:]
	}
	if _, rest, ok := strings.Cut(url, "://"); ok {
		if _, path, found := strings.Cut(rest, "/"); found {
			return "/" + path
		}
		return "/"
	}
	return url
}

func settingInt(s model.Settings, key string) int {
	switch v := s[key].(type) {
	case int64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func settingText(s model.Settings, key string) string {
	switch v := s[key].(type) {
	case nil:
		return ""
	case time.Duration:
		return v.String()
	}
	return fmt.Sprint(s[key])
}

func durationText(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}

func firstText(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
