// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
)

func newTraceCommand(i do.Injector) *cobra.Command {
	var collection, runFile string
	cmd := &cobra.Command{
		Use:   "trace <flow> <name>",
		Short: "Show which step makes a value of a flow and which steps read it",
		Long: "Follow one value through a flow, from the flow as written (no run needed). The name is a variable\n" +
			"(token or vars.token), a flow input (inputs.email) or what a step produced (login.body.token).\n\n" +
			"Exits 1 when no step makes the value and no step reads it, or when a variable or output is read but\n" +
			"nothing makes it: a typo, or a step that was removed.\n\n" +
			"--run <bundle> adds the value at each step, as that run had it (secrets masked). A run bundle is what\n" +
			"flowi run --bundle writes.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrace(cmd, i, args[0], args[1], collection, runFile)
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "collection (name or id) to look in")
	cmd.Flags().StringVar(&runFile, "run", "", "a run bundle (.flowi-run): show the value at each step as that run had it")
	return cmd
}

func runTrace(cmd *cobra.Command, i do.Injector, flow, name, collection, runFile string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindFlow(flow, collection)
	if err != nil {
		return err
	}
	trace, hops, err := traceWithRun(s, target, name, runFile)
	if err != nil {
		return err
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		data, err := json.MarshalIndent(struct {
			analysis.ValueTrace
			Hops []flowi.ValueHop `json:"hops,omitempty"`
		}{trace, hops}, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the trace: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		writeTrace(cmd.OutOrStdout(), trace, hops)
	}
	if traceIsDangling(trace) {
		return Exit(ExitFailed)
	}
	return nil
}

// traceWithRun is the trace of name, with the values of the bundled run when runFile names one.
func traceWithRun(s *Session, target flowi.FlowTarget, name, runFile string) (analysis.ValueTrace, []flowi.ValueHop, error) {
	if runFile == "" {
		trace, err := s.Workspace.TraceValue(target, name)
		return trace, nil, err
	}
	rep, err := readBundleReport(runFile)
	if err != nil {
		return analysis.ValueTrace{}, nil, err
	}
	return s.Workspace.TraceValueInRun(target, name, rep)
}

// traceIsDangling is true for a name that points at nothing: unmade, or made nowhere yet read. A flow input has
// no maker by design.
func traceIsDangling(t analysis.ValueTrace) bool {
	if t.Kind == analysis.TraceInput {
		return false
	}
	return len(t.Producers) == 0
}

func writeTrace(w io.Writer, t analysis.ValueTrace, hops []flowi.ValueHop) {
	fmt.Fprintf(w, "%s (%s)\n", t.Name, t.Kind)
	switch {
	case t.Kind == analysis.TraceInput:
		fmt.Fprintln(w, "  a flow input: the person running the flow supplies it")
	case len(t.Producers) == 0:
		fmt.Fprintln(w, "  nothing makes this value")
	case t.Generated:
		fmt.Fprintf(w, "  made by    %s (generated: fresh, not taken from another step)\n", strings.Join(t.Producers, ", "))
	default:
		fmt.Fprintf(w, "  made by    %s\n", strings.Join(t.Producers, ", "))
	}
	if len(t.Readers) == 0 {
		fmt.Fprintln(w, "  read by    no step")
	} else {
		fmt.Fprintf(w, "  read by    %s\n", strings.Join(t.Readers, ", "))
	}
	writeHops(w, hops)
}

// writeHops prints the value at each step of a traced run.
func writeHops(w io.Writer, hops []flowi.ValueHop) {
	if len(hops) == 0 {
		return
	}
	fmt.Fprintln(w, "  in the run")
	for _, h := range hops {
		value := "(no value in the run)"
		if h.Known {
			value = fmt.Sprintf("%v", h.Value)
		}
		fmt.Fprintf(w, "    %-5s %-16s %s\n", h.Role, h.Node, value)
	}
}
