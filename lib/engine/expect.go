// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"

	"github.com/githugt/flowi/lib/blueprint"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// expectation is the version's expect:, parsed; ok is false when it is malformed (validate reports that; a run
// refuses it rather than guess).
func (x *execution) expectation() (e model.Expectation, ok bool) {
	e, err := model.ParseExpect(x.version.Expect)
	if err != nil {
		x.fail(StatusError, diag.List{diag.New(diag.CodeExpectInvalid, diag.Location{File: x.c.request.File}, map[string]string{"detail": err.Error()})})
		return e, false
	}
	return e, true
}

// checkExpect judges the response's status against the version's expect:. It is a check like the assert lines,
// so it shows with them: `expect: 401` and "got 200".
func (x *execution) checkExpect() {
	if x.version.Expect == "" || x.record == nil {
		return
	}
	e, ok := x.expectation()
	if !ok {
		return
	}
	line := "expect: " + e.String()
	if e.Matches(x.record.Status) {
		x.recordCheck(CheckEval{Line: line, Verdict: CheckPassed})
		return
	}
	got := itoa(x.record.Status)
	x.recordCheck(CheckEval{Line: line, Verdict: CheckFailed, Note: "got " + got})
	x.fail(StatusFailed, diag.List{diag.New(diag.CodeRunExpectFailed, diag.Location{File: x.c.request.File},
		map[string]string{"request": x.c.request.Path, "expected": e.String(), "status": got})})
}

// withoutStatusHook drops the seeded "fail unless 2xx" node from the post hooks of a request that says what it
// expects: the two would contradict each other on a 401 that was meant. The rest of the seeded hook (the log line)
// stays. A hook step that is not exactly the seeded one is the user's own and is left alone.
func (c HookChain) withoutStatusHook() HookChain {
	out := c
	out.Post = nil
	for _, ref := range c.Post {
		if ref.Step.Flow != nil || !slices.ContainsFunc(ref.Step.Inline, blueprint.IsStatusCheck) {
			out.Post = append(out.Post, ref)
			continue
		}
		rest := slices.DeleteFunc(slices.Clone(ref.Step.Inline), blueprint.IsStatusCheck)
		if len(rest) > 0 {
			ref.Step = model.HookStep{Inline: rest}
			out.Post = append(out.Post, ref)
		}
	}
	return out
}
