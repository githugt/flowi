// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

// attributesLine sends flow files to the merge driver named flowi.
const attributesLine = "*.arazzo.yaml merge=flowi"

func newMergeDriverCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge-driver",
		Short: "Let git merge flow files without conflicts on the lines flowi derives",
		Long: "Every edit to a flow rewrites the standard Arazzo steps and a digest of them, which nobody writes by hand, so\n" +
			"two branches that changed different steps of a flow conflicted on those lines. The merge driver merges only\n" +
			"the flow's own data (x-flowi-flow), line by line, and writes the derived parts again from the result. Where\n" +
			"both branches changed the same lines it leaves git's usual markers, and the flow is reported by `flowi validate`.\n\n" +
			"`flowi merge-driver install` switches it on for a collection's repository: a line in .gitattributes (commit\n" +
			"it) and a driver in the repository's git config (which git does not share, so each clone runs install once).",
	}
	cmd.AddCommand(newMergeDriverRun(i), newMergeDriverInstall(i))
	return cmd
}

func newMergeDriverRun(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:    "run <base> <ours> <theirs> <path>",
		Short:  "What git calls: merge three versions of a flow file into <ours>",
		Hidden: true,
		Args:   cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			conflict, err := runMergeDriver(s.Workspace, args[0], args[1], args[2], args[3], flowi.GitLineMerger(cmd.Context()))
			if err != nil {
				return err
			}
			if conflict {
				cmd.PrintErrf("%s: both branches changed the same lines of the flow: resolve the markers, then run `flowi validate`\n", args[3])
				return Exit(ExitFailed)
			}
			return nil
		},
	}
}

// mergeWorkspace is what the driver needs of a workspace.
type mergeWorkspace interface {
	MergeFlowFile(file string, base, ours, theirs []byte, lines flowi.LineMerger) (flowi.MergedFlow, error)
}

// runMergeDriver merges the three files into the ours file, as git expects of a driver. conflict is true when the
// result has conflict markers. A file that is not a flow Flowi wrote (or one it cannot read back after merging) is merged
// as plain text, so installing the driver never makes a merge worse than git's own.
func runMergeDriver(w mergeWorkspace, baseFile, oursFile, theirsFile, path string, lines flowi.LineMerger) (conflict bool, err error) {
	base, errBase := os.ReadFile(baseFile)
	ours, errOurs := os.ReadFile(oursFile)
	theirs, errTheirs := os.ReadFile(theirsFile)
	if err := errors.Join(errBase, errOurs, errTheirs); err != nil {
		return false, err
	}
	merged, err := w.MergeFlowFile(path, base, ours, theirs, lines)
	if err != nil {
		return false, err
	}
	if merged.Clean {
		return false, os.WriteFile(oursFile, merged.Data, 0o644) //nolint:gosec // git's own temp file, read back by git
	}
	text, clean, err := lines(base, ours, theirs)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(oursFile, text, 0o644); err != nil { //nolint:gosec // as above
		return false, err
	}
	return !clean, nil
}

func newMergeDriverInstall(i do.Injector) *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Switch the merge driver on for a collection's repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			id, err := s.Workspace.FindCollection(collection)
			if err != nil {
				return &UsageError{Err: err}
			}
			var root string
			for _, c := range s.Workspace.Collections() {
				if c.Manifest.ID == id {
					root = c.Root
				}
			}
			if root == "" {
				return &UsageError{Err: errors.New("collection not found")}
			}
			added, err := ensureAttributes(root)
			if err != nil {
				return err
			}
			if out, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "config", "--local", "merge.flowi.name", "Flowi flow files").CombinedOutput(); err != nil {
				return fmt.Errorf("git config: %w: %s (is %s in a git repository?)", err, bytes.TrimSpace(out), root)
			}
			if out, err := exec.CommandContext(cmd.Context(), "git", "-C", root, "config", "--local", "merge.flowi.driver", "flowi merge-driver run %O %A %B %P").CombinedOutput(); err != nil {
				return fmt.Errorf("git config: %w: %s", err, bytes.TrimSpace(out))
			}
			if added {
				cmd.Printf("Added %q to %s: commit it.\n", attributesLine, filepath.Join(root, ".gitattributes"))
			} else {
				cmd.Printf("%s already sends flow files to the driver.\n", filepath.Join(root, ".gitattributes"))
			}
			cmd.Println("Registered the driver in this repository's git config; each clone runs `flowi merge-driver install` once.")
			return nil
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "collection (name or id) (default: the only one)")
	return cmd
}

// ensureAttributes makes sure root/.gitattributes sends flow files to the flowi driver, keeping what else it has.
// It reports whether it wrote anything.
func ensureAttributes(root string) (bool, error) {
	file := filepath.Join(root, ".gitattributes")
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == attributesLine {
			return false, nil
		}
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return true, os.WriteFile(file, []byte(text+attributesLine+"\n"), 0o644) //nolint:gosec // a file meant to be committed
}
