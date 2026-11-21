// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/app/internal/vm"
)

// EnvCellView is one variable in one environment.
type EnvCellView struct {
	State  string `json:"state"`  // set, inherited, missing, secret or secret-unset
	Value  string `json:"value"`  // a plain value's text; never a secret's
	Source string `json:"source"` // for an inherited value: where it comes from
}

// EnvCompareRowView is one variable across the environments.
type EnvCompareRowView struct {
	Name    string        `json:"name"`
	Cells   []EnvCellView `json:"cells"`
	Differs bool          `json:"differs"` // the plain values are not all the same
	Gaps    int           `json:"gaps"`    // environments where it is missing or a secret has no value
}

// EnvCompareView is the environments side by side.
type EnvCompareView struct {
	Envs  []string            `json:"envs"`
	Rows  []EnvCompareRowView `json:"rows"`
	Error string              `json:"error,omitempty"`
}

// Compare lays the variables of all the workspace's environments side by side. A secret shows only whether it holds
// a value, never the value.
func (s *EnvironmentService) Compare() EnvCompareView {
	envs := s.app.backend.Environments()
	infos, err := s.app.backend.Secrets() // off the actor: it reads the vault
	m := vm.CompareEnvironments(envs, infos)
	v := EnvCompareView{Envs: m.Envs, Rows: make([]EnvCompareRowView, 0, len(m.Rows))}
	if err != nil {
		v.Error = "Whether secrets hold a value could not be read: " + vm.ErrorText(err)
	}
	for _, r := range m.Rows {
		row := EnvCompareRowView{Name: r.Name, Differs: r.Differs, Gaps: r.Gaps, Cells: make([]EnvCellView, 0, len(r.Cells))}
		for _, c := range r.Cells {
			row.Cells = append(row.Cells, EnvCellView{State: c.State, Value: c.Value, Source: c.Source})
		}
		v.Rows = append(v.Rows, row)
	}
	return v
}
