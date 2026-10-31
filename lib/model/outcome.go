// SPDX-License-Identifier: Apache-2.0

package model

// The two outcomes every step has unless a request says otherwise: it passed (success) or it failed
// (failure: the request, its hooks or its asserts failed, or it could not be sent).
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Outcome is a named way a request can end, which an arrow can follow. A step has every outcome whose rule
// holds once the request and its hooks have run, so a response can be `success` and `ok` at once.
// success and failure are built in: their rule is "the request passed" and "it failed", and they have no
// When. Any other name has a When, an expression over the response (`status in [200, 201]`).
type Outcome struct {
	Name     string
	When     string
	Disabled bool // kept in the list, but never matched
	// Color is one of NodeColors, for the port and arrows of the outcome; "" is the outcome's own (green for
	// success, red for failure, blue for a named one). Description says what it is for, shown beside it.
	Color       string
	Description string
}

// Builtin says whether the outcome is success or failure, whose rule the engine knows.
func (o Outcome) Builtin() bool { return o.Name == OutcomeSuccess || o.Name == OutcomeFailure }

// DefaultOutcomes is the list of a request that writes none: success and failure.
func DefaultOutcomes() []Outcome {
	return []Outcome{{Name: OutcomeSuccess}, {Name: OutcomeFailure}}
}

// EffectiveOutcomes is the version's outcomes: the ones written, or success and failure when it writes none.
// A written list is the whole list: leaving success out removes it, and the steps that follow on success then
// never run. An empty list is the same as none (a request always has an outcome): the editor does not let
// the last one be removed, and a nil list does not survive the trip to the window as different from empty.
func (v RequestVersion) EffectiveOutcomes() []Outcome {
	if len(v.Outcomes) == 0 {
		return DefaultOutcomes()
	}
	return v.Outcomes
}

// ValidOutcomeName says whether name can name an outcome: letters, digits, '-' and '_', not starting with
// '-'. A ':' is how an after_outcome entry joins a step and an outcome, so it cannot be in the name.
func ValidOutcomeName(name string) bool {
	if name == "" || name[0] == '-' {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
