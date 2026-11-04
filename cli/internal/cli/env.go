// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

func newEnvCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments (global to the workspace)",
		Long: "Environments belong to the workspace, not to a collection: every collection sees the same set. They live\n" +
			"in the workspace's environments directory and are not committed with a collection.",
	}
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the workspace's environments",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runEnvLs(cmd, i) },
	}
	diff := &cobra.Command{
		Use:   "diff <a> <b>",
		Short: "Show how two environments differ",
		Long: "Compare two environments as a run would see them (what one extends counts as its own): the variables\n" +
			"only a has (-), only b has (+) and those both have with different values (~). A secret's value is never\n" +
			"shown. Exits 1 when they differ, so a script can check that staging and production stay alike.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error { return runEnvDiff(cmd, i, args[0], args[1]) },
	}
	cmd.AddCommand(ls, diff)
	return cmd
}

func runEnvDiff(cmd *cobra.Command, i do.Injector, a, b string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	cmp, err := s.Workspace.CompareEnvironments(a, b)
	if err != nil {
		return err
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(cmp, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the comparison: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeEnvComparison(cmd.OutOrStdout(), cmp)
	}
	if len(cmp.OnlyA)+len(cmp.OnlyB)+len(cmp.Changed) > 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func writeEnvComparison(out io.Writer, c flowi.EnvComparison) {
	for _, name := range c.OnlyA {
		fmt.Fprintf(out, "- %s  (only in %s)\n", name, c.A)
	}
	for _, name := range c.OnlyB {
		fmt.Fprintf(out, "+ %s  (only in %s)\n", name, c.B)
	}
	for _, ch := range c.Changed {
		fmt.Fprintf(out, "~ %s: %s -> %s\n", ch.Name, ch.A, ch.B)
	}
	if len(c.OnlyA)+len(c.OnlyB)+len(c.Changed) == 0 {
		fmt.Fprintf(out, "%s and %s are the same (%d variables).\n", c.A, c.B, c.Same)
		return
	}
	fmt.Fprintf(out, "%d the same", c.Same)
	if len(c.Secrets) > 0 {
		fmt.Fprintf(out, "; secrets, values not shown: %s", strings.Join(c.Secrets, ", "))
	}
	fmt.Fprintln(out)
}

func runEnvLs(cmd *cobra.Command, i do.Injector) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	envs := s.Workspace.Environments()
	type row struct {
		Name string `json:"name"`
		Vars int    `json:"vars"`
	}
	rows := make([]row, len(envs))
	for n, e := range envs {
		rows[n] = row{Name: e.Name, Vars: len(e.Vars)}
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), rows)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tVARIABLES")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%d\n", r.Name, r.Vars)
	}
	return tw.Flush()
}
