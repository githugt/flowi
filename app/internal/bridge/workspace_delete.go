// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"

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

// DeleteQuestion is what the window asks before a delete: Text is empty when it just goes. Title and Action are empty
// for the plain "Delete"; a delete that breaks flow steps names that on both, so the button is not a plain "Delete".
type DeleteQuestion struct {
	Text, Title, Action string
}

// deleteUse is one flow step that runs a request about to be deleted.
type deleteUse struct{ Request, Where string }

// maxUsesListed is how many steps the question names; the rest are counted.
const maxUsesListed = 6

// callerPlace says where a step is, the way the rest of the window does: "Collection › flow, step name".
func callerPlace(c analysis.Caller) string {
	where := c.Flow
	if where == "" {
		where = "hook " + c.Hook
	}
	return fmt.Sprintf("%s › %s, step %s", c.Collection, where, c.Node)
}

// usesText names the flow steps that would be left without their request. "" when there are none.
//
// Before: deleting a request that flows ran said nothing, and each of them showed "unknown request id" afterwards.
func usesText(uses []deleteUse) string {
	if len(uses) == 0 {
		return ""
	}
	requests := map[string]bool{}
	for _, u := range uses {
		requests[u.Request] = true
	}
	steps := fmt.Sprintf("%d steps", len(uses))
	if len(uses) == 1 {
		steps = "1 step"
	}
	var b strings.Builder
	if len(requests) == 1 {
		fmt.Fprintf(&b, "%s is used by %s:", uses[0].Request, steps)
	} else {
		fmt.Fprintf(&b, "%s run the requests you are deleting:", steps)
	}
	for _, u := range uses[:min(len(uses), maxUsesListed)] {
		if len(requests) == 1 {
			fmt.Fprintf(&b, "\n• %s", u.Where)
		} else {
			fmt.Fprintf(&b, "\n• %s (%s)", u.Where, u.Request)
		}
	}
	if more := len(uses) - maxUsesListed; more > 0 {
		fmt.Fprintf(&b, "\n• and %d more", more)
	}
	b.WriteString("\nThose steps will show an error until you pick another request for them.")
	return b.String()
}

// withUses adds the flow steps a delete would break to the question, and says so on the title and the button. after is
// what is true of this delete once they are named, such as that Undo brings the request back.
func withUses(q DeleteQuestion, uses []deleteUse, after string) DeleteQuestion {
	text := usesText(uses)
	if text == "" {
		return q
	}
	if q.Text != "" {
		text = q.Text + "\n\n" + text
	}
	q.Text = text + after
	q.Title, q.Action = "Delete a request in use", "Delete anyway"
	return q
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
