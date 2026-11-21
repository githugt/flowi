// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/githugt/flowi/lib/flowdiff"
)

type diffFlags struct {
	collection, against, rev string
	exitCode                 bool
}

func newDiffCommand(i do.Injector) *cobra.Command {
	var f diffFlags
	cmd := &cobra.Command{
		Use:   "diff <flow> (--rev <git revision> | --against <file>)",
		Short: "Show what changed in a flow, step by step",
		Long: "Compare a flow with an earlier version of its file and list what changed by step: steps added, removed,\n" +
			"moved, renamed, retyped or rewired, and changes to the flow's inputs, outputs and limits. Steps are matched\n" +
			"by id, so a step taken to another place is one line, not a block of removed and added text.\n\n" +
			"--rev takes the earlier version from git (for example HEAD, main or a commit); --against reads it from a\n" +
			"file. Nothing is written. Exits 0 unless --exit-code is given, which exits 1 when anything changed (as\n" +
			"git diff --exit-code does), 2 for bad usage.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runDiff(cmd, i, args[0], f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.rev, "rev", "", "the git revision holding the earlier version of the flow file")
	fl.StringVar(&f.against, "against", "", "a file holding the earlier version of the flow file")
	fl.BoolVar(&f.exitCode, "exit-code", false, "exit 1 when the flow changed")
	return cmd
}

func runDiff(cmd *cobra.Command, i do.Injector, ref string, f diffFlags) error {
	if (f.rev == "") == (f.against == "") {
		return &UsageError{Err: errors.New("diff wants --rev <git revision> or --against <file>, one of them")}
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindFlow(ref, f.collection)
	if err != nil {
		return err
	}
	var older []byte
	if f.against != "" {
		older, err = os.ReadFile(absPath(do.MustInvoke[Env](i).Cwd, f.against))
		if err != nil {
			return &UsageError{Err: fmt.Errorf("reading %s: %w", f.against, err)}
		}
	} else {
		file, ferr := s.Workspace.FlowFile(target)
		if ferr != nil {
			return ferr
		}
		if older, err = gitShow(cmd.Context(), f.rev, file); err != nil {
			return err
		}
	}
	changes, diags, err := s.Workspace.DiffFlowAgainst(target, older)
	if err != nil {
		return err
	}
	env := do.MustInvoke[Env](i)
	opts := do.MustInvoke[*GlobalOptions](i)
	if err := PrintDiagnostics(cmd.ErrOrStderr(), opts.Output, env.Cwd, diags); err != nil {
		return err
	}
	if len(changes) == 0 && len(diags) > 0 {
		return &UsageError{Err: errors.New("the earlier version could not be read as a flow")}
	}
	if opts.Output == OutputJSON {
		data, err := json.MarshalIndent(changesOrEmpty(changes), "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the changes: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeChanges(cmd.OutOrStdout(), changes)
	}
	if f.exitCode && len(changes) > 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func changesOrEmpty(c []flowdiff.Change) []flowdiff.Change {
	if c == nil {
		return []flowdiff.Change{}
	}
	return c
}

// writeChanges prints one line per change, then how many there were.
func writeChanges(w io.Writer, changes []flowdiff.Change) {
	if len(changes) == 0 {
		fmt.Fprintln(w, "no changes")
		return
	}
	for _, c := range changes {
		fmt.Fprintln(w, "  "+c.String())
	}
	fmt.Fprintf(w, "\n%s\n", plural(len(changes), "change"))
}

// gitShow is the text of file at revision rev. The revision may not start with "-", so it can never be read as an
// option, and git is run with the file's own folder as its place, so the path is relative to it.
func gitShow(ctx context.Context, rev, file string) ([]byte, error) {
	if strings.HasPrefix(rev, "-") {
		return nil, &UsageError{Err: fmt.Errorf("%q is not a git revision", rev)}
	}
	cmd := exec.CommandContext(ctx, "git", "-C", filepath.Dir(file), "show", rev+":./"+filepath.Base(file)) //nolint:gosec // rev cannot start with "-" and goes as one argument, never through a shell
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, &UsageError{Err: fmt.Errorf("git could not show %s at %s: %s", filepath.Base(file), rev, strings.TrimSpace(string(exit.Stderr)))}
		}
		return nil, fmt.Errorf("running git: %w", err)
	}
	return out, nil
}
