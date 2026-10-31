// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi/internal/vm"
)

// MissingVar is a variable a request uses that the selected environment
// (and the request's own variables) do not define.
type MissingVar struct {
	Expr   string `json:"expr"`   // as written between the braces: env.base_url
	Name   string `json:"name"`   // as an environment holds it: base_url; "" for an unknown {{$generator}}, which no environment can define
	Secret bool   `json:"secret"` // used as {{secret.name}}: its value belongs in the vault
}

// MissingView is what a send of the request would lack, so the editor can
// say so before sending rather than after.
type MissingView struct {
	Environment string       `json:"environment"` // the selected environment; "" when none is
	Vars        []MissingVar `json:"vars"`
}

// Missing lists the variables the selected version of request node uses
// that nothing defines yet. A secret counts as defined only once it has a
// value (in the vault, or overridden from the process environment): a
// declaration alone still fails the send.
func (s *RequestService) Missing(node string) MissingView {
	v := MissingView{Vars: []MissingVar{}}
	infos, err := s.app.backend.Secrets() // off the actor: it reads the vault
	s.with(node, func(o *openRequest) {
		vars, secrets := s.app.workspace.VariableNames()
		env := s.app.workspace.SelectedEnv()
		if err == nil { // without the vault's answer, a declaration is the best hint there is
			secrets = withValues(secrets, env, infos)
		}
		cur := o.req.Current()
		known := knownWithOwn(vm.NewNameChecker(vars, secrets), keysOf(cur.Variables))
		v.Environment = env
		v.Vars = missingVars(templatedTexts(cur), known)
	})
	return v
}

// withValues keeps the secrets of env that have a value.
func withValues(declared []string, env string, infos []flowi.SecretInfo) []string {
	var out []string
	for _, name := range declared {
		if slices.ContainsFunc(infos, func(i flowi.SecretInfo) bool { return i.Environment == env && i.Name == name && (i.Set || i.Override) }) {
			out = append(out, name)
		}
	}
	return out
}

// knownWithOwn also counts the request's own variables ({{name}} or {{var.name}}) as defined.
func knownWithOwn(known func(string) bool, own []string) func(string) bool {
	return func(expr string) bool {
		root, rest, _ := strings.Cut(strings.TrimSpace(expr), ".")
		if root == "var" {
			root, _, _ = strings.Cut(rest, ".")
		}
		return slices.Contains(own, root) || known(expr)
	}
}

// templatedTexts is every part of a version a send fills in from variables.
func templatedTexts(v model.RequestVersion) []string {
	texts := []string{v.URL, v.Body.JSON, v.Body.Text, v.Body.File}
	for _, rows := range [][]model.KeyValue{v.Query, v.Headers, v.Body.Form} {
		for _, kv := range rows {
			texts = append(texts, kv.Key, kv.Value)
		}
	}
	if a := v.Async; a != nil {
		texts = append(texts, a.Server, a.Channel)
	}
	if g := v.GRPC; g != nil {
		texts = append(texts, g.Server)
	}
	if d := v.Dataset; d != nil {
		texts = append(texts, d.Server)
		texts = append(texts, d.Params...)
	}
	return texts
}

// missingVars returns the undefined variables in texts, each once, in the
// order they first appear.
func missingVars(texts []string, known func(string) bool) []MissingVar {
	out := []MissingVar{}
	seen := map[string]bool{}
	for _, t := range texts {
		for _, sp := range vm.TemplateSpans(t, known) {
			if sp.Part != vm.PartUnknownVariable {
				continue
			}
			expr := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(sp.Text, "{{"), "}}"))
			if seen[expr] {
				continue
			}
			seen[expr] = true
			out = append(out, missingVarOf(expr))
		}
	}
	return out
}

// missingVarOf names what an environment would need for expr: env.x and a
// bare x are the variable x, secret.x is the secret x, and an unknown $x
// (a misspelt generator) is nothing an environment can hold.
func missingVarOf(expr string) MissingVar {
	if strings.HasPrefix(expr, "$") {
		return MissingVar{Expr: expr}
	}
	root, rest, _ := strings.Cut(expr, ".")
	name, _, _ := strings.Cut(rest, ".")
	switch root {
	case "env":
		return MissingVar{Expr: expr, Name: name}
	case "secret":
		return MissingVar{Expr: expr, Name: name, Secret: true}
	}
	return MissingVar{Expr: expr, Name: root}
}

func keysOf(rows []model.KeyValue) []string {
	out := make([]string, 0, len(rows))
	for _, kv := range rows {
		if kv.Key != "" {
			out = append(out, kv.Key)
		}
	}
	return out
}
