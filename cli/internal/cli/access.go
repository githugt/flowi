// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi-lib"
)

type accessFlags struct {
	collection, env, owner string
	others, steps, inputs  []string
	seed                   uint64
}

func newAccessCommand(i do.Injector) *cobra.Command {
	var f accessFlags
	cmd := &cobra.Command{
		Use:   "access <flow> --owner <identity> --as <identity>",
		Short: "Check that another user cannot reach what a flow made as its owner",
		Long: "Run a flow as its owner, then try its steps as every other identity, with the owner's results in place of\n" +
			"the steps before them, and show what each was answered. A step another user can reach (a 2xx or a\n" +
			"redirect) is a broken access rule; a 401, 403 or 404 is the right answer.\n\n" +
			"An identity is a flowi overlay that makes a request that user's: it sets the Authorization header, a\n" +
			"cookie or an API key. By default the steps tried are the request steps that read something an earlier step\n" +
			"made (a created id), not a login credential; --steps names them instead. Only top-level steps can be tried.\n\n" +
			"Exits 1 when another identity reached something, 2 for bad usage, 4 for a system failure.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runAccess(cmd, i, args[0], f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.env, "env", "", "environment (default: the defaults.environment setting)")
	fl.StringVar(&f.owner, "owner", "", "the identity (an overlay) the flow is first run as")
	fl.StringArrayVar(&f.others, "as", nil, "an identity (an overlay) to try the steps as (repeatable)")
	fl.StringArrayVar(&f.steps, "steps", nil, "the steps to try, instead of the ones that read what an earlier step made (repeatable)")
	fl.StringArrayVar(&f.inputs, "input", nil, "value for a flow input, as name=value (repeatable)")
	fl.Uint64Var(&f.seed, "seed", 0, "seed for generated values (default: random)")
	return cmd
}

func runAccess(cmd *cobra.Command, i do.Injector, ref string, f accessFlags) error {
	if f.owner == "" || len(f.others) == 0 {
		return &UsageError{Err: errors.New("access wants --owner <identity> and at least one --as <identity>")}
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
	rep, err := s.Workspace.CheckAccess(cmd.Context(), target, flowi.AccessOptions{
		Env: f.env, Owner: f.owner, Others: f.others, Steps: f.steps, Inputs: inputs, Seed: f.seed,
	})
	if err != nil {
		return &UsageError{Err: err}
	}
	writeAccess(cmd.OutOrStdout(), rep)
	if rep.Exposed() {
		return Exit(ExitFailed)
	}
	return nil
}

// writeAccess prints a row for each step: its status for the owner, then each other identity's status and verdict.
func writeAccess(w io.Writer, rep flowi.AccessReport) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	header := "step\t" + rep.Owner + " (owner)"
	if len(rep.Steps) > 0 {
		for _, a := range rep.Steps[0].Attempts {
			header += "\t" + a.Identity
		}
	}
	fmt.Fprintln(tw, header)
	for _, st := range rep.Steps {
		row := st.ID + "\t" + strconv.Itoa(st.Owner)
		for _, a := range st.Attempts {
			row += "\t" + verdictGlyph(a.Verdict) + " " + strconv.Itoa(a.Status) + " " + string(a.Verdict)
		}
		fmt.Fprintln(tw, row)
	}
	_ = tw.Flush() // a write to the command's own output
	if rep.Exposed() {
		fmt.Fprintln(w, "\nAnother identity reached what the owner made: the access rule is broken for the steps marked ✗.")
	} else {
		fmt.Fprintln(w, "\nNo other identity reached what the owner made.")
	}
}

func verdictGlyph(v flowi.AccessVerdict) string {
	switch v {
	case flowi.AccessDenied:
		return "✓"
	case flowi.AccessAllowed:
		return "✗"
	}
	return "?"
}
