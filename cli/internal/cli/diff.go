// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/report"
)

func newDiffCommand(i do.Injector) *cobra.Command {
	var timing bool
	cmd := &cobra.Command{
		Use:   "diff <run bundle> <run bundle>",
		Short: "Show how one run of a flow went differently from another",
		Long: "Compare two run bundles (.flowi-run, from flowi run --bundle) step by step: which step first went\n" +
			"differently, which ended with another status, which wrote another value (secrets masked), which ran\n" +
			"in only one of them. Times are left out unless --timing asks for the steps that took twice as long (or\n" +
			"half) and at least 50ms longer or shorter. Use it to ask what a failed CI run did that the last\n" +
			"green one did not.\n\n" +
			"Exits 1 when the runs differ.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error { return runDiff(cmd, i, args[0], args[1], timing) },
	}
	cmd.Flags().BoolVar(&timing, "timing", false, "also list the steps that took much longer or shorter in the second run")
	return cmd
}

func runDiff(cmd *cobra.Command, i do.Injector, first, second string, timing bool) error {
	a, err := readBundleReport(first)
	if err != nil {
		return err
	}
	b, err := readBundleReport(second)
	if err != nil {
		return err
	}
	cmp := report.Compare(a, b)
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(cmp, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the comparison: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		report.WriteComparison(cmd.OutOrStdout(), cmp)
		if timing {
			fmt.Fprintln(cmd.OutOrStdout(), "Timing, against the first run:")
			report.WriteSlowed(cmd.OutOrStdout(), report.SlowedSteps(report.Ghosts(b, a)))
		}
	}
	if len(cmp.Changes) > 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func readBundleReport(path string) (report.Run, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the person names the bundle to read
	if err != nil {
		return report.Run{}, fmt.Errorf("reading the run bundle: %w", err)
	}
	rep, err := flowi.BundleReport(data)
	if err != nil {
		return report.Run{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return rep, nil
}
