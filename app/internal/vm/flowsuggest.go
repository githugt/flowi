// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// SuggestFlows lists the flows Flowi proposes for a collection. They are drafts: nothing is saved.
func (ws *Workspace) SuggestFlows(collection model.ID) ([]flowi.FlowSuggestion, error) {
	return ws.b.SuggestFlows(collection)
}

// AcceptSuggestion saves the suggestion with this path as a flow of the collection and opens it. The suggestion
// is found again rather than passed in: the list is deterministic, and a collection that changed since it was
// shown no longer offers it, which is said rather than saved stale.
func (ws *Workspace) AcceptSuggestion(collection model.ID, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		all, err := ws.b.SuggestFlows(collection)
		if err != nil {
			return "", err
		}
		for _, s := range all {
			if s.Path != path {
				continue
			}
			t, err := ws.b.AcceptSuggestion(collection, s)
			return "f:" + string(t.Collection) + ":" + string(t.Flow), err
		}
		return "", fmt.Errorf("that suggestion is gone: the collection changed since it was shown")
	}, open, fail)
}

// CompareEnvironments says how two environments differ, as a run would see them. Secret values are never in it.
func (ws *Workspace) CompareEnvironments(a, b string) (flowi.EnvComparison, error) {
	return ws.b.CompareEnvironments(a, b)
}

// EnvironmentExtends is the environment that the one named name builds on, "" for none or an unknown name.
func (ws *Workspace) EnvironmentExtends(name string) string {
	for _, e := range ws.b.Environments() {
		if e.Name == name {
			return e.Extends
		}
	}
	return ""
}
