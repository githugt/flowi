// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// matchOutcomes are the names of the outcomes a finished request has, in list order: success when it
// passed, failure when it failed or could not be sent, and each named outcome whose rule holds. A rule
// that cannot be evaluated (it reads a body that is not there) does not match. A disabled outcome never
// matches, and one left out of the list does not exist. A cancelled request has none.
func matchOutcomes(list []model.Outcome, status Status, scope template.Scope) []string {
	if status == StatusCancelled {
		return nil
	}
	out := []string{} // never nil: nil says "not a request", and a request with no outcome left has none
	for _, o := range list {
		if o.Disabled {
			continue
		}
		var holds bool
		switch o.Name {
		case model.OutcomeSuccess:
			holds = status == StatusPassed
		case model.OutcomeFailure:
			holds = status == StatusFailed || status == StatusError
		default:
			ok, _, err := template.Check(o.When, scope)
			holds = err == nil && ok
		}
		if holds {
			out = append(out, o.Name)
		}
	}
	return out
}

// outcomeList is the outcomes the finished request is matched against: the step's own when it has them,
// else the version's.
func (x *execution) outcomeList() []model.Outcome {
	if len(x.c.outcomes) > 0 {
		return x.c.outcomes
	}
	return x.version.EffectiveOutcomes()
}
