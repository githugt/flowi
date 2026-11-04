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
	"github.com/githugt/flowi/lib/discover"
	"github.com/githugt/flowi/lib/grpcx"
)

type discoverFlags struct {
	yes        bool
	tokenEnv   string
	header     string
	rate       int
	paths      []string
	collection string
}

func newDiscoverCommand(i do.Injector) *cobra.Command {
	var f discoverFlags
	cmd := &cobra.Command{
		Use:   "discover <url>",
		Short: "Find out what an API with no description offers, and draft requests from it",
		Long: "Explore an API that has no OpenAPI description: ask a few well-known paths (the root, /health, where specs\n" +
			"are usually served) and follow the same-host paths the answers point to. Only GET is ever sent, every\n" +
			"request says User-Agent: flowi, and a credential goes to the given host only.\n\n" +
			"Because this touches a live server, nothing is sent unless --yes is given: without it the command lists\n" +
			"the requests it would make. With --collection the JSON answers become requests under discovered/, each\n" +
			"expecting the status it got, with the response's shape inferred. Exits 1 when nothing answered.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runDiscover(cmd, i, args[0], f) },
	}
	fl := cmd.Flags()
	fl.BoolVar(&f.yes, "yes", false, "send the requests (without it, only list them)")
	fl.StringVar(&f.tokenEnv, "token-env", "", "name of an environment variable holding the token to send")
	fl.StringVar(&f.header, "header", "Authorization", "header the token travels in; the Authorization header sends it as a Bearer token")
	fl.IntVar(&f.rate, "rate", 4, "requests a second, at most")
	fl.StringArrayVar(&f.paths, "path", nil, "try only this path (repeatable)")
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to save the drafted requests in")
	return cmd
}

func runDiscover(cmd *cobra.Command, i do.Injector, base string, f discoverFlags) error {
	if f.rate < 1 {
		return &UsageError{Err: fmt.Errorf("--rate must be at least 1, got %d", f.rate)}
	}
	if f.collection != "" && !f.yes {
		return &UsageError{Err: fmt.Errorf("--collection saves what was found, so it needs --yes: without it nothing is sent")}
	}
	auth, err := tokenAuth(do.MustInvoke[Env](i), f)
	if err != nil {
		return err
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	asJSON := do.MustInvoke[*GlobalOptions](i).Output == OutputJSON
	if flowi.IsGRPCAddress(base) {
		return runDiscoverGRPC(cmd, s, base, f, asJSON)
	}
	if !f.yes {
		plan, err := s.Workspace.DiscoverPlan(base)
		if err != nil {
			return &UsageError{Err: err}
		}
		return writeDiscoverPlan(cmd.OutOrStdout(), asJSON, plan, f.paths)
	}
	rep, err := s.Workspace.Discover(cmd.Context(), base, auth, f.rate, nilIfEmpty(f.paths))
	if err != nil {
		return err
	}
	var saved flowi.DiscoveredRequests
	if f.collection != "" {
		collection, err := s.Workspace.FindCollection(f.collection)
		if err != nil {
			return err
		}
		if saved, err = s.Workspace.SaveDiscovery(collection, base, rep); err != nil {
			return err
		}
	}
	if err := writeDiscovery(cmd.OutOrStdout(), asJSON, rep, saved, f.collection != ""); err != nil {
		return err
	}
	if !answered(rep) {
		return Exit(ExitFailed)
	}
	return nil
}

// runDiscoverGRPC asks the server which methods it offers and, with --collection, drafts a request for each. Nothing
// is called: reflection only describes the server's own schema.
func runDiscoverGRPC(cmd *cobra.Command, s *Session, server string, f discoverFlags, asJSON bool) error {
	out := cmd.OutOrStdout()
	if !f.yes {
		if asJSON {
			return encodeJSON(out, map[string]any{"dryRun": true, "reflect": server})
		}
		fmt.Fprintf(out, "Dry run: nothing was sent. With --yes %s is asked which services it offers, by reflection; no method is called.\n", server)
		return nil
	}
	methods, err := s.Workspace.DiscoverGRPC(cmd.Context(), server)
	if err != nil {
		return err
	}
	var saved flowi.DiscoveredRequests
	if f.collection != "" {
		collection, err := s.Workspace.FindCollection(f.collection)
		if err != nil {
			return err
		}
		if saved, err = s.Workspace.SaveGRPCDiscovery(collection, server, methods); err != nil {
			return err
		}
	}
	if err := writeGRPCDiscovery(out, asJSON, methods, saved, f.collection != ""); err != nil {
		return err
	}
	if len(methods) == 0 {
		return Exit(ExitFailed)
	}
	return nil
}

func writeGRPCDiscovery(out io.Writer, asJSON bool, methods []grpcx.Method, saved flowi.DiscoveredRequests, wrote bool) error {
	if asJSON {
		return encodeJSON(out, map[string]any{"methods": methods, "created": len(saved.Created), "skipped": saved.Skipped})
	}
	for _, m := range methods {
		fmt.Fprintf(out, "  %s\n", m.Full())
	}
	if wrote {
		fmt.Fprintf(out, "Drafted %d requests under discovered/.\n", len(saved.Created))
		for _, s := range saved.Skipped {
			fmt.Fprintf(out, "  skipped %s\n", s)
		}
	}
	if len(methods) == 0 {
		fmt.Fprintln(out, "The server offers no services.")
	}
	return nil
}

func nilIfEmpty(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	return list
}

// answered reports whether any path gave an answer worth drafting from or reading.
func answered(rep discover.Report) bool {
	for _, f := range rep.Findings {
		if f.Kind != "other" {
			return true
		}
	}
	return false
}

// tokenAuth reads the token named by --token-env from the process environment the command was started with.
func tokenAuth(env Env, f discoverFlags) (discover.Auth, error) {
	if f.tokenEnv == "" {
		return discover.Auth{}, nil
	}
	prefix := f.tokenEnv + "="
	for _, kv := range env.Environ {
		token, ok := strings.CutPrefix(kv, prefix)
		if !ok || token == "" {
			continue
		}
		if strings.EqualFold(f.header, "Authorization") {
			token = "Bearer " + token
		}
		return discover.Auth{Header: f.header, Value: token}, nil
	}
	return discover.Auth{}, &UsageError{Err: fmt.Errorf("--token-env %s names a variable that is not set or is empty", f.tokenEnv)}
}

func writeDiscoverPlan(out io.Writer, asJSON bool, plan []discover.Probe, only []string) error {
	if len(only) > 0 {
		plan = narrowPlan(plan, only)
	}
	if asJSON {
		return encodeJSON(out, map[string]any{"dryRun": true, "probes": plan})
	}
	fmt.Fprintln(out, "Dry run: nothing was sent. With --yes these GET requests are made, and the paths their answers point to:")
	for _, p := range plan {
		fmt.Fprintf(out, "  GET %-36s %s\n", p.Path, p.Why)
	}
	return nil
}

func narrowPlan(plan []discover.Probe, only []string) []discover.Probe {
	var kept []discover.Probe
	for _, p := range plan {
		for _, o := range only {
			if p.Path == o {
				kept = append(kept, p)
			}
		}
	}
	return kept
}

func encodeJSON(out io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the result: %w", err)
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func writeDiscovery(out io.Writer, asJSON bool, rep discover.Report, saved flowi.DiscoveredRequests, wrote bool) error {
	if asJSON {
		return encodeJSON(out, map[string]any{"findings": rep.Findings, "stopped": rep.Stopped, "created": len(saved.Created), "skipped": saved.Skipped})
	}
	for _, f := range rep.Findings {
		fmt.Fprintf(out, "  %-4d %-5s %s\n", f.Status, f.Kind, f.Path)
	}
	if rep.Stopped != "" {
		fmt.Fprintln(out, rep.Stopped+".")
	}
	if wrote {
		fmt.Fprintf(out, "Drafted %d requests under discovered/.\n", len(saved.Created))
		for _, s := range saved.Skipped {
			fmt.Fprintf(out, "  skipped %s\n", s)
		}
	}
	if !answered(rep) {
		fmt.Fprintln(out, "Nothing answered with JSON or an API description.")
	}
	return nil
}
