// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

type baselineFlags struct {
	collection, version string
}

func newBaselineCommand(i do.Injector) *cobra.Command {
	var f baselineFlags
	cmd := &cobra.Command{
		Use:   "baseline <request>",
		Short: "Keep a request's learned schemas as the drift baseline",
		Long: "Schemas flowi inferred from the answers it saw are widened by every later send, so they follow the API wherever\n" +
			"it goes. A discovery draws its schemas from one answer each and calls them shallow. baseline keeps the inferred\n" +
			"schemas of a request as they are: later sends are checked against them instead of widening them, and flowi drift\n" +
			"measures against them. Schemas from a spec or edited by hand are already baselines and are left alone.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runBaseline(cmd, i, args[0], f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.version, "version", "", "version label to keep the schemas of (default: every version)")
	return cmd
}

func runBaseline(cmd *cobra.Command, i do.Injector, ref string, f baselineFlags) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindRequest(ref, f.collection)
	if err != nil {
		return err
	}
	n, err := s.Workspace.PromoteBaseline(target, f.version)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if n == 0 {
		fmt.Fprintln(out, "Nothing to keep: the request has no inferred schema (a spec's or an edited one is a baseline already).")
		return nil
	}
	fmt.Fprintf(out, "Kept %s as the drift baseline.\n", plural(n, "schema"))
	return nil
}
