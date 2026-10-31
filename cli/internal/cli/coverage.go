// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/githugt/flowi/lib/analysis"
)

type coverageFlags struct {
	collection string
	failUnder  int
}

func newCoverageCommand(i do.Injector) *cobra.Command {
	var f coverageFlags
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Show how much of the API descriptions the flows use",
		Long: "For every operation of an imported API description (a request with response schemas), show which flows\n" +
			"call it, which of its documented statuses a saved run ever got, and which response fields a flow reads.\n" +
			"Statuses come from the run history, so a flow that never ran leaves its statuses unseen.\n\n" +
			"--fail-under N exits 1 when fewer than N percent of the operations are used by a flow.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runCoverage(cmd, i, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to measure (default: all mounted)")
	fl.IntVar(&f.failUnder, "fail-under", 0, "exit 1 when fewer than this percent of the operations are used by a flow (0 turns the check off)")
	return cmd
}

func runCoverage(cmd *cobra.Command, i do.Injector, f coverageFlags) error {
	if f.failUnder < 0 || f.failUnder > 100 {
		return &UsageError{Err: fmt.Errorf("--fail-under is a percent from 0 to 100, got %d", f.failUnder)}
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	collection, err := s.Workspace.FindCollection(f.collection)
	if f.collection == "" {
		collection, err = "", nil // every mounted collection
	}
	if err != nil {
		return err
	}
	cov, _, err := s.Workspace.Coverage(collection)
	if err != nil {
		return err
	}
	g := do.MustInvoke[*GlobalOptions](i)
	if g.Output == OutputJSON {
		data, err := json.MarshalIndent(cov, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding coverage: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeCoverage(cmd.OutOrStdout(), cov)
	}
	if t := cov.Totals(); f.failUnder > 0 && t.Operations > 0 && percent(t.Used, t.Operations) < f.failUnder {
		return Exit(ExitFailed)
	}
	return nil
}

func percent(part, whole int) int {
	if whole == 0 {
		return 0
	}
	return part * 100 / whole
}

// writeCoverage prints the used operations with their statuses and fields, the unused ones in a list, and the totals.
func writeCoverage(w io.Writer, cov analysis.Coverage) {
	var unused []analysis.OperationCoverage
	for _, op := range cov.Operations {
		if len(op.Used) == 0 {
			unused = append(unused, op)
			continue
		}
		fmt.Fprintf(w, "%s (version %s)  used by %s\n", op.Request, op.Version, strings.Join(op.Used, ", "))
		fmt.Fprintf(w, "    statuses  %s\n", seenStatuses(op.Statuses))
		fmt.Fprintf(w, "    fields    %d of %d read%s\n", len(op.Read), len(op.Read)+len(op.Unread), unreadNote(op.Unread))
	}
	if len(unused) > 0 {
		fmt.Fprintln(w, "\nNot used by any flow:")
		for _, op := range unused {
			fmt.Fprintf(w, "    %s (version %s)\n", op.Request, op.Version)
		}
	}
	t := cov.Totals()
	fmt.Fprintf(w, "\n%d of %d operations used by a flow (%d%%)\n", t.Used, t.Operations, percent(t.Used, t.Operations))
	fmt.Fprintf(w, "%d of %d documented statuses seen in a run (%d%%)\n", t.SeenStatuses, t.Statuses, percent(t.SeenStatuses, t.Statuses))
	fmt.Fprintf(w, "%d of %d response fields read by a flow (%d%%)\n", t.ReadFields, t.Fields, percent(t.ReadFields, t.Fields))
}

func seenStatuses(statuses []analysis.StatusSeen) string {
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		mark := "✗"
		if s.Seen {
			mark = "✓"
		}
		parts = append(parts, fmt.Sprintf("%d %s", s.Status, mark))
	}
	return strings.Join(parts, "   ")
}

// unreadNote names up to five unread fields, so a long list does not bury the rest.
func unreadNote(unread []string) string {
	if len(unread) == 0 {
		return ""
	}
	shown := unread[:min(len(unread), 5)]
	more := ""
	if len(unread) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(unread)-len(shown))
	}
	return "  (unread: " + strings.Join(shown, ", ") + more + ")"
}
