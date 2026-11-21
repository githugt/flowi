// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/schemainfer"
)

type driftFlags struct {
	collection, env string
	failOn          []string
	inputs          []string
	requests        []string
	update          bool
	seed            uint64
}

func newDriftCommand(i do.Injector) *cobra.Command {
	var f driftFlags
	cmd := &cobra.Command{
		Use:   "drift <flow>",
		Short: "Compare a flow's live responses with the schemas learned before",
		Long: "Run a flow without learning and compare every response with the schema stored for it. It lists what moved:\n" +
			"new fields (the schema does not know them), missing fields (required, now gone) and changed fields (another\n" +
			"type). The stored schemas are not touched, so the next check measures against the same baseline; a normal run\n" +
			"widens them as the API changes, which is how a baseline is lost.\n\n" +
			"--update accepts what the check saw as the new baseline: new fields are added to the stored schemas, a\n" +
			"required field that is gone becomes optional, a field of another type takes both. Commit the changed\n" +
			"*.schemas files. The check then exits 0, since the drift was accepted; --request limits it to some requests.\n\n" +
			"--fail-on says which kinds fail the check (default missing,changed: a new field rarely breaks a client; use\n" +
			"--fail-on new,missing,changed for a strict contract). Exits 1 when a failing kind is found, 2 for bad usage.\n" +
			"Run it from cron or CI to watch an API.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runDrift(cmd, i, args[0], f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.env, "env", "", "environment (default: the defaults.environment setting)")
	fl.StringSliceVar(&f.failOn, "fail-on", []string{"missing", "changed"}, "the kinds of drift that fail the check: new, missing, changed")
	fl.BoolVar(&f.update, "update", false, "accept the drift found as the new baseline: rewrite the stored schemas to agree with the responses")
	fl.StringArrayVar(&f.requests, "request", nil, "with --update, only accept these requests (path; repeatable)")
	fl.StringArrayVar(&f.inputs, "input", nil, "value for a flow input, as name=value (repeatable)")
	fl.Uint64Var(&f.seed, "seed", 0, "seed for generated values (default: random)")
	return cmd
}

func parseDriftKinds(names []string) ([]schemainfer.DriftKind, error) {
	var out []schemainfer.DriftKind
	for _, n := range names {
		switch k := schemainfer.DriftKind(n); k {
		case schemainfer.DriftNew, schemainfer.DriftMissing, schemainfer.DriftChanged:
			out = append(out, k)
		default:
			return nil, &UsageError{Err: fmt.Errorf("--fail-on wants new, missing or changed, got %q", n)}
		}
	}
	return out, nil
}

func runDrift(cmd *cobra.Command, i do.Injector, ref string, f driftFlags) error {
	if len(f.requests) > 0 && !f.update {
		return &UsageError{Err: errors.New("--request only narrows --update")}
	}
	kinds, err := parseDriftKinds(f.failOn)
	if err != nil {
		return err
	}
	inputs, err := parsePairs("--input", f.inputs)
	if err != nil {
		return err
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindFlow(ref, f.collection)
	if err != nil {
		return err
	}
	rep, err := s.Workspace.CheckDrift(cmd.Context(), target, flowi.DriftOptions{Env: f.env, Inputs: inputs, Seed: f.seed})
	if err != nil {
		return err
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding drift: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeDrift(cmd.OutOrStdout(), rep, kinds)
	}
	if f.update {
		return updateDrift(cmd, s.Workspace, rep, f.requests)
	}
	if rep.Has(kinds...) {
		return Exit(ExitFailed)
	}
	return nil
}

// updateDrift writes the drift the check showed into the stored schemas and says what changed; it goes to standard
// error so that --output json on standard output stays one document.
func updateDrift(cmd *cobra.Command, ws *flowi.Workspace, rep flowi.DriftReport, requests []string) error {
	changes, err := ws.AcceptDrift(rep, requests)
	if err != nil {
		return err
	}
	for _, c := range changes {
		fmt.Fprintf(cmd.ErrOrStderr(), "updated %s  %s %s\n", describePart(c.Part), c.Change.Path, c.Change.Kind)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%d schema(s) created, %d change(s) to existing ones: review and commit the *.schemas files\n",
		schemasToCreate(rep, requests), len(changes))
	return nil
}

// schemasToCreate counts the responses the check saw that had no stored schema (accepting gives each one), among
// the requests accepted.
func schemasToCreate(rep flowi.DriftReport, requests []string) int {
	n := 0
	for _, q := range rep.Requests {
		if q.NoSchema && (len(requests) == 0 || slices.Contains(requests, q.Request)) {
			n++
		}
	}
	return n
}

func describePart(p flowi.SchemaPart) string {
	if p.Kind == "response" {
		return "response " + strconv.Itoa(p.Status)
	}
	return p.Kind
}

// writeDrift prints each request that was called, with what moved in its response. A kind that fails the check
// is marked ✗ and one that does not !.
func writeDrift(w io.Writer, rep flowi.DriftReport, failing []schemainfer.DriftKind) {
	fmt.Fprintf(w, "%s · the run %s\n", rep.Flow, rep.RunStatus)
	drifted := 0
	for _, q := range rep.Requests {
		fmt.Fprintf(w, "%s (version %s) · %d\n", q.Request, q.Version, q.Status)
		switch {
		case q.NoSchema:
			fmt.Fprintln(w, "    no schema is stored for this response yet: send it with schema learning on, then check again")
		case len(q.Differences) == 0:
			fmt.Fprintln(w, "    no drift")
		}
		for _, d := range q.Differences {
			drifted++
			mark := "!"
			if slices.Contains(failing, d.Kind) {
				mark = "✗"
			}
			line := fmt.Sprintf("    %s %-8s %s", mark, d.Kind, d.Path)
			if d.Detail != "" {
				line += "   " + d.Detail
			}
			fmt.Fprintln(w, line)
		}
	}
	fmt.Fprintln(w, "\n"+strconv.Itoa(drifted)+" difference(s) from the stored schemas")
}
