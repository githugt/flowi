// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/watch"
)

type watchFlags struct {
	collection, env string
}

func newWatchCommand(i do.Injector) *cobra.Command {
	var f watchFlags
	cmd := &cobra.Command{
		Use:   "watch <flow>",
		Short: "Run a read-only flow and say what changed since the last watch",
		Long: "Run a flow that only reads (every step is a GET, HEAD or OPTIONS) and compare what its calls answered with\n" +
			"the last pass: another status, a response whose shape moved (a field gone, new or of another type), a call\n" +
			"more than three times slower. The first pass becomes the baseline. Exits 1 when something changed or the\n" +
			"run failed, so a schedule or a CI job can alert on it. A flow with a step that may change something is\n" +
			"refused, with the reasons: it would be run again and again, unattended.\n\n" +
			"`watch add`, `ls` and `rm` keep watches that `flowi serve` runs on a cron schedule: a watch is a schedule\n" +
			"whose runs are watch passes, and it is listed by `flowi schedule ls` too.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runWatch(cmd, i, args[0], f) },
	}
	bindWatchRunFlags(cmd, &f)
	cmd.AddCommand(newWatchRunCommand(i), newWatchAddCommand(i), newWatchLsCommand(i), newWatchRmCommand(i))
	return cmd
}

func bindWatchRunFlags(cmd *cobra.Command, f *watchFlags) {
	cmd.Flags().StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	cmd.Flags().StringVar(&f.env, "env", "", "environment to run in")
}

func newWatchRunCommand(i do.Injector) *cobra.Command {
	var f watchFlags
	cmd := &cobra.Command{
		Use:   "run <flow>",
		Short: "Run a read-only flow once and say what changed (same as `flowi watch <flow>`)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return runWatch(cmd, i, args[0], f) },
	}
	bindWatchRunFlags(cmd, &f)
	return cmd
}

func newWatchAddCommand(i do.Injector) *cobra.Command {
	var f scheduleFlags
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Watch a read-only flow on a schedule",
		Long: "Save a schedule whose runs are watch passes. `flowi serve` runs it at the times --cron gives (see\n" +
			"`flowi schedule add --help` for the syntax); each pass is compared with the one before, and a pass that\n" +
			"finds the contract moved ends as \"changed\" in the run record. A flow with a step that may change\n" +
			"something is refused, with the reasons. Restart `flowi serve` after changing watches.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f.watch = true
			return runScheduleAdd(cmd, i, args[0], f)
		},
	}
	bindScheduleFlags(cmd, &f)
	return cmd
}

func newWatchLsCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List the watches and when each runs next",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			watching := slices.DeleteFunc(s.Workspace.Schedules(), func(x model.Schedule) bool { return !x.Watch })
			rows := scheduleRows(watching, time.Now())
			for n := range rows {
				history, err := s.Workspace.ScheduleWatchHistory(watching[n])
				if err != nil {
					return err
				}
				rows[n].Last = lastPass(history)
			}
			if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
				return writeJSONLines(cmd.OutOrStdout(), rows)
			}
			return writeWatchTable(cmd.OutOrStdout(), rows)
		},
	}
}

// lastPass says in a word how the latest pass went.
func lastPass(history []watch.Pass) string {
	if len(history) == 0 {
		return "not yet"
	}
	p := history[len(history)-1]
	switch {
	case p.Status != string(engine.StatusPassed):
		return p.Status
	case p.Changes > 0:
		return fmt.Sprintf("changed (%d)", p.Changes)
	}
	return "ok"
}

func writeWatchTable(w io.Writer, rows []scheduleRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tCRON\tNEXT\tFLOW\tLAST")
	for _, r := range rows {
		next := r.Next
		if r.Disabled {
			next = "(disabled)"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Cron, next, r.Flow, r.Last)
	}
	return tw.Flush()
}

func newWatchRmCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Stop watching: remove a watch",
		Long:  "Remove a watch. A schedule that is not a watch is left alone: remove it with `flowi schedule rm`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			name := args[0]
			if isPlainSchedule(s.Workspace.Schedules(), name) {
				return &UsageError{Err: fmt.Errorf("%s is a schedule, not a watch: remove it with `flowi schedule rm %s`", name, name)}
			}
			if err := s.Workspace.RemoveSchedule(name); err != nil {
				return err
			}
			cmd.Printf("Removed watch %s\n", name)
			return nil
		},
	}
}

// isPlainSchedule reports that the list has a schedule called name that does not watch.
func isPlainSchedule(list []model.Schedule, name string) bool {
	return slices.ContainsFunc(list, func(x model.Schedule) bool { return x.Name == name && !x.Watch })
}

func runWatch(cmd *cobra.Command, i do.Injector, flow string, f watchFlags) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindFlow(flow, f.collection)
	if err != nil {
		return err
	}
	problems, err := s.Workspace.WatchProblems(target)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return &UsageError{Err: fmt.Errorf("%s cannot be watched: %v", flow, problems)}
	}
	res, err := s.Workspace.WatchFlow(cmd.Context(), target, flowi.RunOptions{Env: f.env})
	if err != nil {
		return err
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the watch result: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeWatch(cmd.OutOrStdout(), res)
	}
	if res.Status != engine.StatusPassed || len(res.Change) > 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func writeWatch(out io.Writer, r flowi.WatchResult) {
	switch {
	case r.Status != engine.StatusPassed:
		fmt.Fprintf(out, "The run %s, so nothing was compared and the last baseline is kept.\n", r.Status)
	case r.First:
		fmt.Fprintf(out, "First pass: %d requests recorded as the baseline.\n", r.Steps)
	case len(r.Change) == 0:
		fmt.Fprintf(out, "No change in %d requests.\n", r.Steps)
	}
	for _, c := range r.Change {
		fmt.Fprintf(out, "%-8s %s  %s\n", c.Kind, c.Request, c.Detail)
	}
}
