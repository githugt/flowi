// SPDX-License-Identifier: Apache-2.0

package model

import "strings"

// Arrow is a line into a step: the step it leaves, and the outcomes of that step that take it. Success and
// failure have flags; any other outcome of the step's request is named in On.
type Arrow struct {
	From      string
	OnSuccess bool
	OnFailure bool
	On        []string
}

// Takes says whether the outcome (success, failure or a name from the request's outcomes) takes the arrow.
func (a Arrow) Takes(outcome string) bool {
	switch outcome {
	case OutcomeSuccess:
		return a.OnSuccess
	case OutcomeFailure:
		return a.OnFailure
	}
	for _, o := range a.On {
		if o == outcome {
			return true
		}
	}
	return false
}

// Arrows are the lines into n. previous is the id of the step before n in its list ("" for the first):
// a step with no needs, after_failure_of or after_outcome follows it, on success. A step with any arrow of
// its own has only those; it no longer follows the step before it.
func (n Node) Arrows(previous string) []Arrow {
	if len(n.Needs) == 0 && len(n.AfterFailureOf) == 0 && len(n.AfterOutcome) == 0 {
		if previous == "" || n.Unlinked {
			return nil
		}
		return []Arrow{{From: previous, OnSuccess: true}}
	}
	var out []Arrow
	at := map[string]int{}
	add := func(id, outcome string) {
		i, seen := at[id]
		if !seen {
			i = len(out)
			at[id] = i
			out = append(out, Arrow{From: id})
		}
		switch outcome {
		case OutcomeSuccess:
			out[i].OnSuccess = true
		case OutcomeFailure:
			out[i].OnFailure = true
		default:
			if !out[i].Takes(outcome) {
				out[i].On = append(out[i].On, outcome)
			}
		}
	}
	for _, id := range n.Needs {
		add(id, OutcomeSuccess)
	}
	for _, id := range n.AfterFailureOf {
		add(id, OutcomeFailure)
	}
	for _, entry := range n.AfterOutcome {
		if id, outcome, ok := strings.Cut(entry, ":"); ok {
			add(id, outcome)
		}
	}
	return out
}
