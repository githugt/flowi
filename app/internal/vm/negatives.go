// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"slices"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// SuggestNegatives lists the ways the request on a tree row can be sent wrong, as overlays that are drafts: nothing
// is saved until AcceptNegatives. The first version of the request is the one that is broken.
func (ws *Workspace) SuggestNegatives(node string) ([]model.Overlay, error) {
	t, err := ws.requestOfRow(node)
	if err != nil {
		return nil, err
	}
	return ws.b.SuggestNegatives(t, "")
}

// AcceptNegatives saves the named variants as overlays. The list is made again rather than passed in, as for
// AcceptSuggestion: a request that changed since it was shown no longer offers a variant, which is said rather than
// saved stale. A name already taken is left as it is and comes back in skipped.
func (ws *Workspace) AcceptNegatives(node string, names []string) (saved, skipped []string, err error) {
	t, err := ws.requestOfRow(node)
	if err != nil {
		return nil, nil, err
	}
	all, err := ws.b.SuggestNegatives(t, "")
	if err != nil {
		return nil, nil, err
	}
	var chosen []model.Overlay
	for _, name := range names {
		i := slices.IndexFunc(all, func(o model.Overlay) bool { return o.Name == name })
		if i < 0 {
			return nil, nil, fmt.Errorf("the variant %q is gone: the request changed since it was shown", name)
		}
		chosen = append(chosen, all[i])
	}
	return ws.b.AcceptNegatives(t.Collection, chosen)
}

func (ws *Workspace) requestOfRow(node string) (flowi.RequestTarget, error) {
	n := ws.Node(node)
	if n.Kind != NodeRequest {
		return flowi.RequestTarget{}, errors.New("negative variants are made for a request: choose a request row")
	}
	return flowi.RequestTarget{Collection: n.Collection, Request: n.Request}, nil
}
