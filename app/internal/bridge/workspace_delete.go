// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"

	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/vm"
)

// Deleted is the "deleted" event: requests or flows that went to the trash, so the window can offer Undo for a few
// seconds. IDs are the trash items.
type Deleted struct {
	Text string   `json:"text"`
	IDs  []string `json:"ids"`
}

// deletedNotice is what the window says after items went to the trash: the name for one, the count for several.
func deletedNotice(items []flowi.TrashedItem) Deleted {
	d := Deleted{IDs: make([]string, 0, len(items))}
	for _, it := range items {
		d.IDs = append(d.IDs, it.ID)
	}
	switch len(items) {
	case 0:
	case 1:
		name := items[0].Name
		if name == "" {
			name = items[0].Path
		}
		d.Text = fmt.Sprintf("Deleted %s", name)
	default:
		d.Text = fmt.Sprintf("Deleted %d items", len(items))
	}
	return d
}

// leafDeleteQuestion is what to ask before deleting rows that are not folders. Requests and flows go to the trash
// and can be undone, so they are not asked about; an environment is removed for good, so it is.
//
// Before: every delete asked "This cannot be undone", including the ones that now can be.
func leafDeleteQuestion(envNames []string, others int, effects []string) string {
	if len(envNames) == 0 {
		return ""
	}
	const maxNamed = 5
	list := strings.Join(envNames[:min(len(envNames), maxNamed)], ", ")
	if more := len(envNames) - maxNamed; more > 0 {
		list += fmt.Sprintf(" and %d more", more)
	}
	q := fmt.Sprintf("Delete %s? This cannot be undone.", list)
	if len(effects) > 0 {
		q += " " + strings.Join(effects, " ")
	}
	if others > 0 {
		q += " The other selected items go to the Trash."
	}
	return q
}

// envEffects says what deleting an environment takes along or leaves hanging: its stored secret values go with it, and
// an environment that builds on it loses what it gave. Nothing to say gives "".
//
// Before: the question named the environment and nothing of what depended on it.
func envEffects(name string, impact flowi.EnvImpact) string {
	var parts []string
	if n := len(impact.Secrets); n > 0 {
		what := "secret"
		if n > 1 {
			what = "secrets"
		}
		parts = append(parts, fmt.Sprintf("%s's %d %s and their stored values go with it (%s).", name, n, what, strings.Join(impact.Secrets, ", ")))
	}
	if n := len(impact.Dependents); n > 0 {
		verb := "builds"
		if n > 1 {
			verb = "build"
		}
		parts = append(parts, fmt.Sprintf("%s %s on %s and would lose what it gives.", strings.Join(impact.Dependents, ", "), verb, name))
	}
	return strings.Join(parts, " ")
}

// Undo puts items that a delete sent to the trash back where they were. It returns a problem to show, or "".
func (s *TrashService) Undo(ids []string) string {
	var problem string
	for _, id := range ids {
		if _, err := s.app.backend.RestoreFromTrash(id); err != nil && problem == "" {
			problem = vm.ErrorText(err)
		}
	}
	s.changed()
	return problem
}
