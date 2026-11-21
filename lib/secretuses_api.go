// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"cmp"
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// The kinds of file that can use a secret.
const (
	UseRequest = "request"
	UseFlow    = "flow"
	UseAuth    = "auth"
)

// SecretUse is a file that reads a secret: {{secret.name}} written in a request, a flow or a named authentication.
type SecretUse struct {
	Kind       string
	Name       string // the request's or flow's path, or the authentication's name
	Collection model.ID
	ID         model.ID // the request's or flow's id; empty for an authentication
}

// SecretUses says, for each secret name, which requests, flows and named authentications of the mounted collections
// write {{secret.name}} (found in their text, so a name only built by an expression is not seen). Each list is sorted
// by kind and name.
//
// Before: the Secrets screen showed a name and a state, and nothing said what would break if the value were removed
// or what a leftover secret was still used by.
func (w *Workspace) SecretUses() map[string][]SecretUse {
	out := map[string][]SecretUse{}
	add := func(text string, u SecretUse) {
		for _, name := range namesIn(secretReadPattern, text) {
			out[name] = append(out[name], u)
		}
	}
	for _, c := range w.Collections() {
		id := c.Manifest.ID
		for _, r := range c.Requests {
			add(jsonText(r), SecretUse{Kind: UseRequest, Name: r.Path, Collection: id, ID: r.ID})
		}
		for _, f := range c.Flows {
			add(jsonText(f), SecretUse{Kind: UseFlow, Name: f.Path, Collection: id, ID: f.ID})
		}
		for _, a := range c.Auths {
			add(jsonText(a), SecretUse{Kind: UseAuth, Name: a.Name, Collection: id})
		}
	}
	for name := range out {
		slices.SortFunc(out[name], func(a, b SecretUse) int {
			return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Name, b.Name))
		})
	}
	return out
}
