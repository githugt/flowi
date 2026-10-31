// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"
)

type impactFlags struct {
	collection, version, against, spec string
}

func newImpactCommand(i do.Injector) *cobra.Command {
	var f impactFlags
	cmd := &cobra.Command{
		Use:   "impact <request> | --against <spec>",
		Short: "Show which flows and hooks use a request, and whether they still fit it",
		Long: "List every consumer of a request (by path, e.g. auth/login, or id) across all mounted\n" +
			"collections: flow request nodes, post-hooks, and the request's own extract and assert.\n" +
			"Each is checked against the request's contract (its pinned examples, else its last\n" +
			"saved response): ✓ compatible, ⚠ unknown, ✗ broken.\n\n" +
			"With --against <spec> (a file or URL), check what a changed OpenAPI description would break,\n" +
			"before you import it: each flow, hook and check that uses a request is judged by the response\n" +
			"schemas the new description gives, and everything that uses a request the new description\n" +
			"dropped is broken. Only what the change breaks is listed, not what was broken before. The\n" +
			"description is compared with the one the collection kept when it was imported (--spec names it\n" +
			"when there are several). Nothing is written.\n\n" +
			"Exits 1 when any consumer is broken.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runImpactCommand(cmd, i, args, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.version, "version", "", "report only this version")
	fl.StringVar(&f.against, "against", "", "check consumers against this OpenAPI description (a file or URL) instead of the saved contracts")
	fl.StringVar(&f.spec, "spec", "", "with --against: the name of the kept description to compare with (default: the only one)")
	return cmd
}

func runImpactCommand(cmd *cobra.Command, i do.Injector, args []string, f impactFlags) error {
	switch {
	case len(args) == 1 && f.against != "":
		return &UsageError{Err: errors.New("name a request or use --against, not both")}
	case len(args) == 0 && f.against == "":
		return &UsageError{Err: errors.New("impact wants a request or --against <spec>")}
	case f.against != "":
		return runImpactAgainst(cmd, i, f)
	}
	return runImpact(cmd, i, args[0], f)
}

func runImpact(cmd *cobra.Command, i do.Injector, ref string, f impactFlags) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	target, err := s.Workspace.FindRequest(ref, f.collection)
	if err != nil {
		return err
	}
	rep, diags, err := s.Workspace.Impact(target, f.version)
	if err != nil {
		return err
	}
	g := do.MustInvoke[*GlobalOptions](i)
	if err := PrintDiagnostics(cmd.ErrOrStderr(), g.Output, do.MustInvoke[Env](i).Cwd, diags); err != nil {
		return err
	}
	if err := writeImpact(cmd, g.Output, rep); err != nil {
		return err
	}
	if rep.Status == analysis.Broken {
		return Exit(ExitFailed)
	}
	return nil
}

func writeImpact(cmd *cobra.Command, output string, rep analysis.Report) error {
	if output != OutputJSON {
		writeImpactText(cmd.OutOrStdout(), rep)
		return nil
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding impact report: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

// runImpactAgainst checks what a changed description would break.
func runImpactAgainst(cmd *cobra.Command, i do.Injector, f impactFlags) error {
	env := do.MustInvoke[Env](i)
	src, err := importSource(cmd, flowi.ImportOpenAPI, f.against, env.Cwd)
	if err != nil {
		return err
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	collection, err := s.Workspace.FindCollection(f.collection)
	if err != nil {
		return err
	}
	res, diags, err := s.Workspace.ImpactAgainst(collection, f.spec, src.Data)
	if err != nil {
		return err
	}
	g := do.MustInvoke[*GlobalOptions](i)
	if err := PrintDiagnostics(cmd.ErrOrStderr(), g.Output, env.Cwd, diags); err != nil {
		return err
	}
	if err := writeSpecImpact(cmd, g.Output, res); err != nil {
		return err
	}
	if res.Broken() {
		return Exit(ExitFailed)
	}
	return nil
}
