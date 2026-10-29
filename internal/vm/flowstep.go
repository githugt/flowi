// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// SetColor gives step id one of the colours the library knows ("" for its
// category's own). The colour is cosmetic; a run never reads it.
func (d *FlowDoc) SetColor(id, color string) error {
	if color != "" && !slices.Contains(model.NodeColors(), color) {
		return fmt.Errorf("%q is not a colour a step can have (%v)", color, model.NodeColors())
	}
	if _, ok := d.Node(id); !ok {
		return fmt.Errorf("no step %s", id)
	}
	d.EditNode(id, func(n *model.Node) { n.Color = color })
	return nil
}

// Duplicate puts a copy of step id, with the steps inside it, right after it
// and returns the copy's id. The copy and everything inside it get free ids;
// what points between the copied steps follows the new ids.
func (d *FlowDoc) Duplicate(id string) (string, error) {
	var copyID string
	err := d.command(func(f *model.Flow) error {
		path, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no step %s", id)
		}
		list, _ := listAt(f, path)
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		fresh := freshNodes((*list)[i:i+1], taken)
		copyID = fresh[0].ID
		*list = slices.Insert(*list, i+1, fresh[0])
		return nil
	})
	return copyID, err
}

// AddRouteByStatus puts a condition right after step id that takes a path by
// that step's response status: one case for 200, one for 404, one for any 5xx,
// and the else lane for everything else. It returns the condition's id. Each
// case is an ordinary case the person can edit.
func (d *FlowDoc) AddRouteByStatus(id string) (string, error) {
	if _, ok := d.Node(id); !ok {
		return "", fmt.Errorf("no step %s", id)
	}
	status := "nodes." + id + ".status"
	cases := []model.Case{
		{When: status + " == 200"},
		{When: status + " == 404"},
		{When: status + " >= 500 && " + status + " < 600"},
	}
	var made string
	err := d.command(func(f *model.Flow) error {
		path, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no step %s", id)
		}
		list, _ := listAt(f, path)
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		n := newNode("condition", taken)
		n.Cases = cases
		made = n.ID
		*list = slices.Insert(*list, i+1, n)
		return nil
	})
	return made, err
}
