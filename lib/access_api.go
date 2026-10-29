// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/flowioverlay"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

// AccessOptions configure CheckAccess. An identity is a flowi overlay that makes the requests that user's: it
// sets the Authorization header, a cookie or an API key.
type AccessOptions struct {
	Env    string
	Owner  string   // the identity the flow is first run as; what it makes belongs to it
	Others []string // the identities that must not reach what the owner made
	// Steps are the top-level request steps to try as each other identity. Empty chooses the steps that read
	// something an earlier step made (a created id, say), apart from credentials.
	Steps  []string
	Inputs map[string]string
	Seed   uint64
}

// AccessVerdict is what one attempt shows.
type AccessVerdict string

// Verdicts. A refusal (401, 403 or 404) is what the owner's data should get from another user; a success
// means the other user reached it; anything else (a 400, a server error, no answer) proves neither.
const (
	AccessDenied  AccessVerdict = "denied"
	AccessAllowed AccessVerdict = "allowed"
	AccessUnclear AccessVerdict = "unclear"
)

// AccessAttempt is one step tried as one other identity.
type AccessAttempt struct {
	Identity string
	Status   int // 0 when the step got no answer
	Verdict  AccessVerdict
}

// AccessStep is one step: its status for the owner, then what each other identity got.
type AccessStep struct {
	ID       string
	Owner    int
	Attempts []AccessAttempt
}

// AccessReport is the result of CheckAccess.
type AccessReport struct {
	Flow  string
	Owner string
	Steps []AccessStep
}

// Exposed says whether any other identity reached what the owner made.
func (r AccessReport) Exposed() bool {
	return slices.ContainsFunc(r.Steps, func(s AccessStep) bool {
		return slices.ContainsFunc(s.Attempts, func(a AccessAttempt) bool { return a.Verdict == AccessAllowed })
	})
}

// credentialLike names the values that pass a login on, not an object: a later step reading one is not
// reading another user's data.
var credentialLike = regexp.MustCompile(`(?i)token|auth|session|jwt|bearer|cookie|secret|key|password`)

// CheckAccess runs the flow as the owner, then tries each chosen step as every other identity, with the
// owner's results in place of the steps before it, and reports what each was answered. A step the other user
// can reach is a broken access rule: the check that "B cannot read A's data".
func (w *Workspace) CheckAccess(ctx context.Context, t FlowTarget, o AccessOptions) (AccessReport, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return AccessReport{}, err
	}
	if err := checkIdentities(c, o); err != nil {
		return AccessReport{}, err
	}
	steps := accessSteps(f, o.Steps)
	if len(steps) == 0 {
		return AccessReport{}, errors.New("no step reads what an earlier step made, so there is nothing to try as another user: name the steps to try")
	}
	runOpts := RunOptions{Env: o.Env, Inputs: o.Inputs, Seed: o.Seed, Overlay: o.Owner}
	owner, err := w.runQuietly(ctx, t, runOpts)
	if err != nil {
		return AccessReport{}, err
	}
	if owner.Status != engine.StatusPassed {
		return AccessReport{}, fmt.Errorf("the flow does not pass as %s (%s), so there is nothing to compare: fix that run first", o.Owner, owner.Status)
	}
	rep := AccessReport{Flow: f.Path, Owner: o.Owner}
	for _, id := range steps {
		step, err := w.tryStep(ctx, t, runOpts, owner, id, o.Others)
		if err != nil {
			return rep, err
		}
		rep.Steps = append(rep.Steps, step)
	}
	return rep, nil
}

// checkIdentities refuses an identity that is not an overlay of the collection, before anything runs.
func checkIdentities(c model.Collection, o AccessOptions) error {
	for _, name := range append([]string{o.Owner}, o.Others...) {
		if name == "" {
			return errors.New("the owner identity is needed: the overlay the flow is first run as")
		}
		if _, missing := flowioverlay.Resolve(c.Overlays, name, ""); missing != "" {
			return diagErr(diag.CodeOverlayNotFound, map[string]string{"name": missing})
		}
	}
	if len(o.Others) == 0 {
		return errors.New("name at least one other identity to try")
	}
	return nil
}

// accessSteps are the steps to try: the ones named, or the top-level request steps that read what an earlier
// step made, apart from credentials.
func accessSteps(f model.Flow, named []string) []string {
	if len(named) > 0 {
		return named
	}
	requests := map[string]bool{}
	for _, n := range f.Nodes {
		if n.Type == "request" {
			requests[n.ID] = true
		}
	}
	var out []string
	for _, l := range analysis.DataLinks(f) {
		if requests[l.To] && requests[l.From] && !credentialLike.MatchString(l.Via) && !slices.Contains(out, l.To) {
			out = append(out, l.To)
		}
	}
	return out
}

func (w *Workspace) runQuietly(ctx context.Context, t FlowTarget, o RunOptions) (engine.RunResult, error) {
	run, err := w.RunFlow(ctx, t, o)
	if err != nil {
		return engine.RunResult{}, err
	}
	for range run.Events() { //nolint:revive // nothing is shown: draining lets the run end
	}
	return run.Wait(), nil
}

// tryStep runs one step as each other identity, standing on the owner's results for the steps before it.
func (w *Workspace) tryStep(ctx context.Context, t FlowTarget, base RunOptions, owner engine.RunResult, id string, others []string) (AccessStep, error) {
	step := AccessStep{ID: id, Owner: statusOf(owner.Report.Nodes, id)}
	for _, identity := range others {
		opts := base
		opts.Overlay, opts.Only = identity, []string{id}
		opts.Prior, opts.PriorVars = outputsOf(owner.Report.Nodes, id), owner.Report.Vars
		opts.Seed = owner.Seed
		res, err := w.runQuietly(ctx, t, opts)
		if err != nil {
			return step, err
		}
		status := statusOf(res.Report.Nodes, id)
		step.Attempts = append(step.Attempts, AccessAttempt{Identity: identity, Status: status, Verdict: verdictOf(status)})
	}
	return step, nil
}

// outputsOf are the outputs of every top-level step before the one named: what a partial run stands on.
func outputsOf(nodes []report.Node, before string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, n := range nodes {
		if n.ID == before {
			break
		}
		out[n.ID] = n.Outputs
	}
	return out
}

// statusOf is the HTTP status a request step ended with, or 0.
func statusOf(nodes []report.Node, id string) int {
	for _, n := range nodes {
		if n.ID != id {
			continue
		}
		switch v := n.Outputs["status"].(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func verdictOf(status int) AccessVerdict {
	switch {
	case status == 401 || status == 403 || status == 404:
		return AccessDenied
	case status >= 200 && status < 400:
		return AccessAllowed
	}
	return AccessUnclear
}
