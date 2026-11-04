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

type explainFlags struct {
	collection string
	problems   bool
	format     string
}

func newExplainCommand(i do.Injector) *cobra.Command {
	var f explainFlags
	cmd := &cobra.Command{
		Use:   "explain <flow>",
		Short: "Tell a flow in words: what it does, step by step, and what looks wrong with it",
		Long: "Print a flow as sentences, one per step, with branches, loops and parallel lanes indented below the\n" +
			"line that starts them. It reads the flow as written: no run, no network. The text has no colour and\n" +
			"nothing is cut short, so it can be pasted into a review, a ticket or an AI assistant as it is.\n\n" +
			"Below the sentences it lists problems: a variable a step sets that nothing reads, and a variable a step\n" +
			"reads that no step sets. --problems prints only those, and exits 1 when there are any.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runExplain(cmd, i, args[0], f) },
	}
	cmd.Flags().StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	cmd.Flags().BoolVar(&f.problems, "problems", false, "print only the problems; exit 1 when there are any")
	cmd.Flags().StringVar(&f.format, "format", "text", "text or markdown")
	return cmd
}

func runExplain(cmd *cobra.Command, i do.Injector, flow string, f explainFlags) error {
	if f.format != "text" && f.format != "markdown" {
		return &UsageError{Err: fmt.Errorf("--format must be text or markdown, got %q", f.format)}
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindFlow(flow, f.collection)
	if err != nil {
		return err
	}
	explained, err := s.Workspace.ExplainFlow(target)
	if err != nil {
		return err
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(explained, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the explanation: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeExplanation(cmd.OutOrStdout(), explained, f)
	}
	if f.problems && len(explained.Problems) > 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func writeExplanation(out io.Writer, e analysis.Explanation, f explainFlags) {
	markdown := f.format == "markdown"
	if !f.problems {
		for _, s := range e.Sentences {
			fmt.Fprintln(out, sentenceLine(s, markdown))
		}
	}
	if len(e.Problems) == 0 {
		if f.problems {
			fmt.Fprintln(out, "No problems found.")
		}
		return
	}
	if !f.problems {
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, heading("Problems", markdown))
	for _, p := range e.Problems {
		fmt.Fprintf(out, "%s%s (step %s)\n", bullet(markdown), p.Text, p.Node)
	}
}

// sentenceLine is one sentence, indented two spaces (or, in markdown, a list item nested the same way).
func sentenceLine(s analysis.Sentence, markdown bool) string {
	indent := strings.Repeat("  ", s.Indent)
	if markdown {
		return indent + "- " + s.Text
	}
	return indent + s.Text
}

func heading(text string, markdown bool) string {
	if markdown {
		return "## " + text
	}
	return text + ":"
}

func bullet(markdown bool) string {
	if markdown {
		return "- "
	}
	return "  - "
}
