// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/app/internal/vm"
)

// EnvCellView is one variable in one environment.
type EnvCellView struct {
	State  string `json:"state"`  // set, inherited, missing, secret or secret-unset
	Value  string `json:"value"`  // a plain value's text; never a secret's
	Kind   string `json:"kind"`   // how a plain value is read: "" text, "number", "boolean" or "json"; "" for the rest
	Source string `json:"source"` // for an inherited value: where it comes from
}

// EnvCompareRowView is one variable across the environments.
type EnvCompareRowView struct {
	Name        string        `json:"name"`
	Cells       []EnvCellView `json:"cells"`
	Differs     bool          `json:"differs"`     // what the environments define is not all the same: text, type, or plain against secret
	KindsDiffer bool          `json:"kindsDiffer"` // the plain values are not all read as the same type: show each cell's type
	Gaps        int           `json:"gaps"`        // environments where it is missing or a secret has no value
}

// EnvGroupView is one group of environments.
type EnvGroupView struct {
	Name  string `json:"name"`  // "" is the default group
	Label string `json:"label"` // what to show
	Count int    `json:"count"` // how many environments are in it
}

// EnvCompareView is the environments of one group side by side.
type EnvCompareView struct {
	Group  string              `json:"group"`  // the group compared; "" is the default group
	Groups []EnvGroupView      `json:"groups"` // every group, for the picker
	Envs   []string            `json:"envs"`
	Rows   []EnvCompareRowView `json:"rows"`
	Notes  []string            `json:"notes"` // what the table cannot show by itself, such as a base in another group
	Error  string              `json:"error,omitempty"`
}

// Compare lays the variables of the workspace's environments in one group side by side ("" is the default group). A
// secret shows only whether it holds a value, never the value.
func (s *EnvironmentService) Compare(group string) EnvCompareView {
	envs := s.app.backend.Environments()
	infos, err := s.app.backend.Secrets() // off the actor: it reads the vault
	m := vm.CompareGroup(envs, group, infos)
	v := EnvCompareView{Group: group, Groups: groupViews(vm.ManageEnvironments(envs, s.app.backend.EnvironmentGroups()).Groups), Envs: m.Envs, Rows: make([]EnvCompareRowView, 0, len(m.Rows)), Notes: nonNil(m.Notes)}
	if err != nil {
		v.Error = "Whether secrets hold a value could not be read: " + vm.ErrorText(err)
	}
	for _, r := range m.Rows {
		row := EnvCompareRowView{Name: r.Name, Differs: r.Differs, KindsDiffer: r.KindsDiffer, Gaps: r.Gaps, Cells: make([]EnvCellView, 0, len(r.Cells))}
		for _, c := range r.Cells {
			row.Cells = append(row.Cells, EnvCellView{State: c.State, Value: c.Value, Kind: c.Kind, Source: c.Source})
		}
		v.Rows = append(v.Rows, row)
	}
	return v
}

func groupViews(groups []vm.EnvGroup) []EnvGroupView {
	out := make([]EnvGroupView, 0, len(groups))
	for _, g := range groups {
		out = append(out, EnvGroupView{Name: g.Name, Label: g.Label, Count: g.Count})
	}
	return out
}
