// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func delaySpec() Spec {
	return Spec{
		Type: "delay", Group: "Control", Summary: "Wait a fixed or random time (2s, 1s..3s).",
		Params: []Param{{Name: "duration", Kind: ParamText, Required: true, Doc: `A duration ("2s") or a range ("1s..3s") picked with the run's seed; may be a template.`}},
		Run:    runDelay,
	}
}

// runDelay waits through the runtime, so cancellation and test clocks
// apply.
func runDelay(ctx context.Context, n model.Node, s *State) Outcome {
	raw, _ := param(n, "duration")
	v, err := eval(raw, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "duration", err)}
	}
	d, err := pickDuration(template.String(v), s.Runtime)
	if err != nil {
		return failed(n, diag.CodeNodeParam, map[string]string{"field": "duration", "detail": err.Error()})
	}
	if err := s.Runtime.Sleep(ctx, d); err != nil {
		return Outcome{Status: StatusCancelled}
	}
	return Outcome{}
}

// ParseDelay reads "2s" or "lo..hi" (lo ≤ hi); shared with validate.
func ParseDelay(text string) (lo, hi time.Duration, err error) {
	loText, hiText, isRange := strings.Cut(text, "..")
	if lo, err = time.ParseDuration(strings.TrimSpace(loText)); err != nil {
		return 0, 0, fmt.Errorf("%q is not a duration (use 2s or 1s..3s)", text)
	}
	if !isRange {
		return lo, lo, nil
	}
	if hi, err = time.ParseDuration(strings.TrimSpace(hiText)); err != nil || hi < lo {
		return 0, 0, fmt.Errorf("%q is not a range from low to high (use 1s..3s)", text)
	}
	return lo, hi, nil
}

func pickDuration(text string, rt Runtime) (time.Duration, error) {
	lo, hi, err := ParseDelay(text)
	if err != nil || lo == hi {
		return lo, err
	}
	return rt.RandomDuration(lo, hi), nil
}
