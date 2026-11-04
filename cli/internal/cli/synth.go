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
	"github.com/githugt/flowi/lib/model"
)

type synthFlags struct {
	collection    string
	negatives     string
	version       string
	write         bool
	minConfidence float64
}

func newSynthCommand(i do.Injector) *cobra.Command {
	var f synthFlags
	cmd := &cobra.Command{
		Use:   "synth",
		Short: "Suggest flows from the requests of a collection and what their responses return",
		Long: "Read the HTTP requests of a collection and find the ones that make something another request takes in\n" +
			"its path (POST /pets returns an id, GET /pets/{petId} takes it). For each resource that has such a\n" +
			"pair, propose a flow that creates, reads, updates and deletes it, and say which response field feeds\n" +
			"which parameter and how sure the match is. Nothing is written unless --write is given; then each\n" +
			"proposal is saved as a new flow under flows/suggested/ and an existing flow is never overwritten.\n\n" +
			"What a response holds is read from the import's response schemas, pinned examples or the last saved\n" +
			"response, so a collection with none of those gets no proposals.\n\n" +
			"With --negatives <request> it instead lists the ways to send that request wrong, as flowi overlays: each\n" +
			"required field dropped (no-<field>), each field of the wrong type (bad-<field>) and an expired token\n" +
			"(stale-token). --write saves them; run one with flowi send <request> --flowi-overlay no-email, or all of\n" +
			"them with flowi run --matrix. What the API should answer is yours to assert.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runSynth(cmd, i, f) },
	}
	cmd.Flags().StringVar(&f.collection, "collection", "", "collection (name or id) to read")
	cmd.Flags().StringVar(&f.negatives, "negatives", "", "suggest overlays that send this request wrong, instead of flows")
	cmd.Flags().StringVar(&f.version, "version", "", "with --negatives: the version label to break (default: the first)")
	cmd.Flags().BoolVar(&f.write, "write", false, "save the proposals as flows")
	cmd.Flags().Float64Var(&f.minConfidence, "min-confidence", 0, "only propose flows whose every link is at least this sure (0 to 1)")
	return cmd
}

func runSynth(cmd *cobra.Command, i do.Injector, f synthFlags) error {
	if f.minConfidence < 0 || f.minConfidence > 1 {
		return &UsageError{Err: fmt.Errorf("--min-confidence must be between 0 and 1, got %v", f.minConfidence)}
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	if f.negatives != "" {
		return runNegatives(cmd, s.Workspace, f)
	}
	collection, err := s.Workspace.FindCollection(f.collection)
	if err != nil {
		return err
	}
	all, err := s.Workspace.SuggestFlows(collection)
	if err != nil {
		return err
	}
	suggestions := sureEnough(all, f.minConfidence)
	saved, err := saveSuggestions(s.Workspace, collection, suggestions, f.write)
	if err != nil {
		return err
	}
	return writeSynth(cmd.OutOrStdout(), do.MustInvoke[*GlobalOptions](i).Output == OutputJSON, suggestions, saved)
}

// sureEnough keeps the suggestions whose every parameter link has at least floor confidence.
func sureEnough(all []flowi.FlowSuggestion, floor float64) []flowi.FlowSuggestion {
	var kept []flowi.FlowSuggestion
	for _, s := range all {
		if lowestConfidence(s) >= floor {
			kept = append(kept, s)
		}
	}
	return kept
}

func lowestConfidence(s flowi.FlowSuggestion) float64 {
	lowest := 1.0
	for _, step := range s.Steps {
		for _, t := range step.Takes {
			lowest = min(lowest, t.Confidence)
		}
	}
	return lowest
}

// saveSuggestions writes each suggestion when write is set and returns the paths of the flows it made, in order.
func saveSuggestions(w *flowi.Workspace, collection model.ID, list []flowi.FlowSuggestion, write bool) ([]string, error) {
	if !write {
		return nil, nil
	}
	var paths []string
	for _, s := range list {
		target, err := w.AcceptSuggestion(collection, s)
		if err != nil {
			return paths, fmt.Errorf("saving %s: %w", s.Path, err)
		}
		paths = append(paths, savedFlowPath(w, target))
	}
	return paths, nil
}

// savedFlowPath is the path of the flow t names, for telling the person where a suggestion went.
func savedFlowPath(w *flowi.Workspace, t flowi.FlowTarget) string {
	for _, c := range w.Collections() {
		for _, f := range c.Flows {
			if f.ID == t.Flow {
				return "flows/" + f.Path
			}
		}
	}
	return string(t.Flow)
}

func writeSynth(out io.Writer, asJSON bool, list []flowi.FlowSuggestion, saved []string) error {
	if asJSON {
		data, err := json.MarshalIndent(struct {
			Suggestions []flowi.FlowSuggestion `json:"suggestions"`
			Saved       []string               `json:"saved,omitempty"`
		}{list, saved}, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding the suggestions: %w", err)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(list) == 0 {
		fmt.Fprintln(out, "Nothing to suggest: no request makes a value that another request takes in its path.")
		return nil
	}
	for n, s := range list {
		fmt.Fprintf(out, "%s  (flows/%s)\n", s.Name, s.Path)
		for _, step := range s.Steps {
			fmt.Fprintf(out, "  %-8s %-6s %s\n", step.ID, step.Method, step.Request)
			for _, t := range step.Takes {
				fmt.Fprintf(out, "           %s from %s.%s  (%.0f%% sure)\n", t.Param, t.From, t.Field, t.Confidence*100)
			}
		}
		if n < len(saved) {
			fmt.Fprintf(out, "  saved as %s\n", saved[n])
		}
	}
	return nil
}

// runNegatives lists the overlays that send a request wrong and, with --write, saves the ones whose names are free.
func runNegatives(cmd *cobra.Command, w *flowi.Workspace, f synthFlags) error {
	target, err := w.FindRequest(f.negatives, f.collection)
	if err != nil {
		return err
	}
	overlays, err := w.SuggestNegatives(target, f.version)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(overlays) == 0 {
		fmt.Fprintln(out, "Nothing to break: the request has no JSON object body and no Authorization header.")
		return nil
	}
	for _, ov := range overlays {
		fmt.Fprintf(out, "  %-24s %s\n", ov.Name, ov.Description)
	}
	if !f.write {
		fmt.Fprintln(out, "Nothing was written. With --write these are saved as flowi overlays.")
		return nil
	}
	saved, skipped, err := w.AcceptNegatives(target.Collection, overlays)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Saved %s.", plural(len(saved), "overlay"))
	if len(skipped) > 0 {
		fmt.Fprintf(out, " Left alone, the name is taken: %s.", strings.Join(skipped, ", "))
	}
	fmt.Fprintln(out)
	return nil
}
