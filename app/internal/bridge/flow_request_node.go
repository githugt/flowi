// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// RequestNode is the tree node of the request a step calls, so the window can open it beside the flow. A step that
// calls no request (a delay, a variable set) has none, and a request that cannot be found says so.
func (s *FlowService) RequestNode(node, step string) (string, error) {
	var out string
	err := s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(step)
		if !ok {
			return fmt.Errorf("no step %q", step)
		}
		if n.Type != "request" || n.Request == nil {
			return fmt.Errorf("step %q does not call a request", step)
		}
		c, ok := s.app.workspace.Collection(s.app.workspace.Node(node).Collection)
		if !ok {
			return fmt.Errorf("the collection of %q is not loaded", node)
		}
		ref := n.Request.Request
		i := slices.IndexFunc(c.Requests, func(r model.Request) bool {
			return (ref.ID != "" && r.ID == ref.ID) || (ref.Path != "" && r.Path == ref.Path)
		})
		if i < 0 {
			return fmt.Errorf("step %q calls a request that cannot be found", step)
		}
		out = "r:" + string(c.Manifest.ID) + ":" + string(c.Requests[i].ID)
		return nil
	})
	return out, err
}
