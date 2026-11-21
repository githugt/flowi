// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/githugt/flowi/lib/diag"
)

func newValidateCommand(i do.Injector) *cobra.Command {
	var env string
	cmd := &cobra.Command{
		Use:   "validate [dir]",
		Short: "Check collections for errors",
		Long: "Check every mounted collection, or the collection in dir (mounted or not), and report problems.\n\n" +
			"With --env, also warn about every flow that reads a variable or secret the environment does not have, so a new " +
			"teammate or a CI job learns before a run what is missing there.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := do.Invoke[*Session](i)
			if err != nil {
				return err
			}
			diags, err := validateTarget(s, args)
			if err != nil {
				return err
			}
			if env != "" && len(args) == 0 {
				needs, err := s.Workspace.PreflightAll(env)
				if err != nil {
					return err
				}
				diags = append(diags, needs...)
			}
			g := do.MustInvoke[*GlobalOptions](i)
			process := do.MustInvoke[Env](i)
			if err := PrintDiagnostics(cmd.OutOrStdout(), g.Output, process.Cwd, diags); err != nil {
				return err
			}
			// In a GitHub Actions job the problems also become annotations on the files.
			if g.Output != OutputGitHub && wantsAnnotations(g.Output, process.Environ) {
				if err := PrintDiagnostics(cmd.OutOrStdout(), OutputGitHub, process.Cwd, diags); err != nil {
					return err
				}
			}
			cmd.PrintErrln(summarise(diags))
			if diags.HasErrors() {
				return Exit(ExitUsage)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "also check what this environment lacks for the flows (variables and secrets they read)")
	return cmd
}

// validateTarget validates the given directory, or the whole workspace
// (whose results already include the problems found while opening it).
func validateTarget(s *Session, args []string) (diag.List, error) {
	if len(args) == 1 {
		return s.Workspace.ValidateDir(args[0])
	}
	return s.Workspace.Validate(), nil
}
