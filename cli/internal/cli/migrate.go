// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

func newMigrateCommand(i do.Injector) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "migrate [--check]",
		Short: "Show whether collections are in the file format this release writes",
		Long: "Report, for every mounted collection, whether its files are in the format this release reads and writes,\n" +
			"and whether the release is new enough for the collection (min_version in its flowi.yaml). Nothing is written:\n" +
			"no format upgrade exists yet, because format 1 is the only one.\n\n" +
			"With --check, exits 1 when any collection is outdated, was written by a newer release, or needs a newer\n" +
			"release than this one, so a CI job can fail early with a clear reason.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			report := s.Workspace.Migrations()
			if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
				if report == nil {
					report = []flowi.Migration{}
				}
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding the report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				writeMigrations(cmd.OutOrStdout(), report)
			}
			if check && !allCurrent(report) {
				return Exit(ExitFailed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "exit 1 when a collection is not current")
	return cmd
}

func allCurrent(report []flowi.Migration) bool {
	for _, m := range report {
		if m.State != "current" {
			return false
		}
	}
	return true
}

// writeMigrations prints one block per collection.
func writeMigrations(w io.Writer, report []flowi.Migration) {
	if len(report) == 0 {
		fmt.Fprintln(w, "no collections are mounted")
		return
	}
	for _, m := range report {
		fmt.Fprintf(w, "%s: format %d, %s", m.Collection, m.Format, m.State)
		switch m.State {
		case "too new":
			fmt.Fprintf(w, " (this release writes format %d: upgrade Flowi)", m.Current)
		case "needs newer release":
			fmt.Fprintf(w, " (min_version %s: upgrade Flowi)", m.MinVersion)
		}
		fmt.Fprintln(w)
		for _, s := range m.Steps {
			fmt.Fprintf(w, "    %s\n", s)
		}
		if m.Problem != "" {
			fmt.Fprintf(w, "    cannot upgrade: %s\n", m.Problem)
		}
	}
}
