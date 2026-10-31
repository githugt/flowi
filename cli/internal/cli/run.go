// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/cli/internal/buildinfo"
	"github.com/githugt/flowi/cli/internal/tui"
)

type runFlags struct {
	collection, env, prefer, tag, bundle, overlay string
	matrix                                        []string
	inputs, reports                               []string
	seed                                          uint64
	grace                                         time.Duration
	quiet                                         bool
}

func newRunCommand(i do.Injector) *cobra.Command {
	var f runFlags
	cmd := &cobra.Command{
		Use:   "run <flow> | --tag <tag>",
		Short: "Run a flow, or every flow with a tag",
		Long: "Run a flow (by path, e.g. user-creation, or id) and show each node as it finishes.\n\n" +
			"With --tag, run every flow that carries the tag (its own `tags:` or its folder's), one after\n" +
			"the other; the exit code is the worst of their outcomes. --report writes one file per flow,\n" +
			"with the flow's path added to the name (out.xml becomes out-user-creation.xml).\n\n" +
			"Exit codes: 0 passed, 1 a node failed, 2 bad usage or input, 3 cancelled or aborted,\n" +
			"4 a transport or system failure (including a report that could not be written).\n" +
			"Every run's report is saved in the workspace; --report also writes it as junit, json,\n" +
			"markdown or text, as a Mermaid sequence diagram of the calls, or as a HAR file of the requests and\n" +
			"responses (secrets masked).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runRun(cmd, i, args, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.collection, "collection", "", "collection (name or id) to look in")
	fl.StringVar(&f.env, "env", "", "environment (default: the defaults.environment setting)")
	fl.StringVar(&f.bundle, "bundle", "", "also write the run as a bundle (.flowi-run): the flow, seed, inputs and every call, secrets masked; `flowi run <bundle>` plays it back offline")
	fl.StringVar(&f.overlay, "overlay", "", "lay this flowi overlay (or a stack, vip+slow) over every request step that names none of its own")
	fl.StringArrayVar(&f.matrix, "matrix", nil, "run the flow once for every combination, as dimension=value,value: env, overlay or input.<name> (repeatable); prints a grid of passes and failures")
	fl.StringVar(&f.tag, "tag", "", "run every flow that carries this tag instead of naming one")
	fl.StringVar(&f.prefer, "prefer-version", "", "run this version of each request that has it")
	fl.StringArrayVar(&f.inputs, "input", nil, "value for a flow input, as name=value (a template; repeatable)")
	fl.Uint64Var(&f.seed, "seed", 0, "seed for generated values (default: random, printed)")
	fl.DurationVar(&f.grace, "grace", 0, "how long a cancelled run may take to stop (default: the run.grace setting)")
	fl.StringArrayVar(&f.reports, "report", nil, "also write the report, as format=path: junit, json, markdown, text, mermaid (a sequence diagram) or har (repeatable)")
	fl.BoolVar(&f.quiet, "quiet", false, "print only the final status line")
	return cmd
}

func runRun(cmd *cobra.Command, i do.Injector, args []string, f runFlags) error {
	if err := f.checkTarget(args); err != nil {
		return err
	}
	opts, reports, err := f.options()
	if err != nil {
		return err
	}
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	if f.tag != "" {
		return runTagged(cmd, i, s, opts, reports, f)
	}
	if flowi.IsBundleRef(args[0]) {
		return runBundle(cmd, i, s, args[0], reports, f)
	}
	if flowi.IsArazzoRef(args[0]) {
		return runArazzo(cmd, i, s, args[0], opts, reports, f)
	}
	target, err := s.Workspace.FindFlow(args[0], f.collection)
	if err != nil {
		return err
	}
	if len(f.matrix) > 0 {
		return runMatrix(cmd, s, target, opts, reports, f)
	}
	return runOne(cmd, i, s, target, opts, reports, f.bundle, f)
}

// checkTarget wants a flow or a tag, not both and not neither, and a matrix only for one flow of its own.
func (f runFlags) checkTarget(args []string) error {
	switch {
	case len(f.matrix) > 0 && (f.tag != "" || len(args) == 1 && (flowi.IsArazzoRef(args[0]) || flowi.IsBundleRef(args[0]))):
		return &UsageError{Err: errors.New("--matrix runs one saved flow; it does not combine with --tag, an Arazzo file or a bundle")}
	case len(f.matrix) > 0 && (f.env != "" || f.overlay != ""):
		return &UsageError{Err: errors.New("--matrix sets the environment and the overlay itself; leave --env and --overlay out, or list them as dimensions")}
	case len(args) == 1 && f.tag != "":
		return &UsageError{Err: errors.New("name a flow or use --tag, not both")}
	case len(args) == 0 && f.tag == "":
		return &UsageError{Err: errors.New("run wants a flow or --tag <tag>")}
	}
	return nil
}

// runBundle plays a bundled run back from its recording. It decides the environment, seed and inputs itself,
// so options that would change them are refused rather than ignored.
func runBundle(cmd *cobra.Command, i do.Injector, s *Session, file string, reports []reportSpec, f runFlags) error {
	for flag, set := range map[string]bool{"--env": f.env != "", "--seed": f.seed != 0, "--input": len(f.inputs) > 0, "--prefer-version": f.prefer != "", "--bundle": f.bundle != "", "--overlay": f.overlay != "", "--matrix": len(f.matrix) > 0} {
		if set {
			return &UsageError{Err: fmt.Errorf("%s does not apply to a replay: it uses the recorded environment, seed and inputs, and a replay is not bundled again", flag)}
		}
	}
	data, err := os.ReadFile(file) //nolint:gosec // the person names the bundle to play
	if err != nil {
		return &UsageError{Err: fmt.Errorf("reading %s: %w", file, err)}
	}
	br, err := s.Workspace.OpenBundle(data)
	if err != nil {
		return &UsageError{Err: err}
	}
	defer br.Close()
	m := br.Manifest
	cmd.PrintErrf("Replaying %s (%s) as recorded %s: environment %s, seed %d. No request is sent.\n", m.FlowName, m.FlowPath, m.At.Format("2006-01-02 15:04"), m.Env, m.Seed)
	return runOne(cmd, i, s, br.Target, br.Options, reports, "", f)
}

// runArazzo runs the workflows of an Arazzo document that is not part of any collection: its OpenAPI
// sources become a temporary collection that is gone when the command ends. A document with one
// workflow keeps the report names as given; with several, each report gets the workflow's name.
func runArazzo(cmd *cobra.Command, i do.Injector, s *Session, ref string, opts flowi.RunOptions, reports []reportSpec, f runFlags) error {
	doc, err := s.Workspace.OpenArazzo(cmd.Context(), ref)
	if err != nil {
		return &UsageError{Err: err}
	}
	defer doc.Close()
	if err := PrintDiagnostics(cmd.ErrOrStderr(), OutputText, do.MustInvoke[Env](i).Cwd, doc.Diags); err != nil {
		return err
	}
	if opts.Env == "" && !f.envSet() {
		opts.Env = doc.Env
	}
	return runTargets(cmd, i, s, doc.Flows, opts, reports, f, len(doc.Flows) > 1)
}

// envSet says whether the person chose an environment, by flag or setting.
func (f runFlags) envSet() bool { return f.env != "" }

// runTagged runs each flow with the tag.
func runTagged(cmd *cobra.Command, i do.Injector, s *Session, opts flowi.RunOptions, reports []reportSpec, f runFlags) error {
	targets, err := s.Workspace.FindFlowsTagged(f.tag, f.collection)
	if err != nil {
		return err
	}
	return runTargets(cmd, i, s, targets, opts, reports, f, true)
}

// runTargets runs the flows one after the other and ends with the worst exit code, so one failure
// does not hide behind a later pass. nameReports adds each flow's path to its report file names.
func runTargets(cmd *cobra.Command, i do.Injector, s *Session, targets []flowi.FlowTarget, opts flowi.RunOptions, reports []reportSpec, f runFlags, nameReports bool) error {
	worst := ExitOK
	for _, target := range targets {
		specs := reports
		if nameReports {
			specs = reportsFor(reports, flowPath(s, target))
		}
		err := runOne(cmd, i, s, target, opts, specs, bundlePathFor(f.bundle, flowPath(s, target), nameReports), f)
		if err == nil {
			continue
		}
		var exit *ExitError
		if !errors.As(err, &exit) {
			return err
		}
		worst = max(worst, exit.Code)
	}
	if worst != ExitOK {
		return Exit(worst)
	}
	return nil
}

func runOne(cmd *cobra.Command, i do.Injector, s *Session, target flowi.FlowTarget, opts flowi.RunOptions, reports []reportSpec, bundlePath string, f runFlags) error {
	run, err := s.Workspace.RunFlow(cmd.Context(), target, opts)
	if err != nil {
		return err
	}
	res := renderRun(cmd, i, run, f)
	logRun(i, target, res)
	return finishRun(cmd, i, res, errors.Join(writeReports(reports, res), writeBundle(s, bundlePath, target, opts, res)))
}

// bundlePathFor adds the flow's path to the bundle's name when several flows are run: runs.flowi-run
// becomes runs-billing-refund.flowi-run.
func bundlePathFor(path, flow string, several bool) string {
	if path == "" || !several {
		return path
	}
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "-" + strings.ReplaceAll(flow, "/", "-") + ext
}

// writeBundle writes the run as a bundle when one was asked for.
func writeBundle(s *Session, path string, target flowi.FlowTarget, opts flowi.RunOptions, res engine.RunResult) error {
	if path == "" {
		return nil
	}
	out, err := os.Create(path) //nolint:gosec // the user names the bundle file on the command line
	if err != nil {
		return fmt.Errorf("writing the bundle: %w", err)
	}
	err = s.Workspace.WriteBundle(out, target, opts, res)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("writing the bundle %s: %w", path, err)
	}
	return nil
}

// flowPath is the path of the flow a target names, or its id when the flow is gone.
func flowPath(s *Session, t flowi.FlowTarget) string {
	for _, c := range s.Workspace.Collections() {
		if c.Manifest.ID != t.Collection {
			continue
		}
		for _, f := range c.Flows {
			if f.ID == t.Flow {
				return f.Path
			}
		}
	}
	return string(t.Flow)
}

// reportsFor adds the flow's path to each report's file name: out.xml becomes out-billing-refund.xml.
func reportsFor(specs []reportSpec, flow string) []reportSpec {
	out := make([]reportSpec, 0, len(specs))
	suffix := "-" + strings.ReplaceAll(flow, "/", "-")
	for _, s := range specs {
		ext := filepath.Ext(s.path)
		out = append(out, reportSpec{format: s.format, path: strings.TrimSuffix(s.path, ext) + suffix + ext})
	}
	return out
}

// options turns flags into run options; bad values are usage errors.
func (f runFlags) options() (flowi.RunOptions, []reportSpec, error) {
	inputs, err := parsePairs("--input", f.inputs)
	if err != nil {
		return flowi.RunOptions{}, nil, err
	}
	reports, err := parseReports(f.reports)
	if err != nil {
		return flowi.RunOptions{}, nil, err
	}
	if f.grace < 0 {
		return flowi.RunOptions{}, nil, &UsageError{Err: errors.New("--grace must not be negative")}
	}
	return flowi.RunOptions{Env: f.env, PreferVersion: f.prefer, Inputs: inputs, Seed: f.seed, Grace: f.grace, Overlay: f.overlay}, reports, nil
}

func parsePairs(flag string, pairs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, &UsageError{Err: fmt.Errorf("%s wants name=value, got %q", flag, p)}
		}
		out[k] = v
	}
	return out, nil
}

// reportSpec is one --report format=path.
type reportSpec struct{ format, path string }

func reportWriters() map[string]func(io.Writer, report.Run) error {
	return map[string]func(io.Writer, report.Run) error{
		"junit": report.WriteJUnit, "json": report.WriteJSON, "markdown": report.WriteMarkdown, "text": report.WriteText,
		"mermaid": report.WriteMermaid,
	}
}

// harFormat is the one report that needs the whole result, not only the report: it holds the requests
// and responses as they went over the wire (secrets masked).
const harFormat = "har"

func parseReports(flags []string) ([]reportSpec, error) {
	writers := reportWriters()
	out := make([]reportSpec, 0, len(flags))
	for _, f := range flags {
		format, path, ok := strings.Cut(f, "=")
		if _, known := writers[format]; !ok || (!known && format != harFormat) || path == "" {
			return nil, &UsageError{Err: fmt.Errorf("--report wants format=path with format junit, json, markdown, text, mermaid or har, got %q", f)}
		}
		out = append(out, reportSpec{format: format, path: path})
	}
	return out, nil
}

// writeReports writes every requested report, even after one fails.
func writeReports(specs []reportSpec, res engine.RunResult) error {
	var errs []error
	for _, s := range specs {
		errs = append(errs, writeReport(s, res))
	}
	return errors.Join(errs...)
}

func writeReport(s reportSpec, res engine.RunResult) error {
	f, err := os.Create(s.path) //nolint:gosec // the user names the report file on the command line
	if err != nil {
		return fmt.Errorf("writing %s report: %w", s.format, err)
	}
	err = writeFormat(f, s.format, res)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("writing %s report %s: %w", s.format, s.path, err)
	}
	return nil
}

// renderRun shows the run's progress and returns its result: NDJSON, live
// progress on a terminal (the plain record is printed after), or plain
// lines.
func renderRun(cmd *cobra.Command, i do.Injector, run *engine.Run, f runFlags) engine.RunResult {
	g := do.MustInvoke[*GlobalOptions](i)
	console := do.MustInvoke[Console](i)
	p := runText{out: cmd.OutOrStdout(), err: cmd.ErrOrStderr(), quiet: f.quiet}
	switch tui.ChooseRenderer(g.Output, f.quiet, console.OutTTY, console.ErrTTY) {
	case tui.RendererJSON:
		res, err := writeRunNDJSON(cmd.OutOrStdout(), run)
		if err != nil {
			cmd.PrintErrf("warning: %v\n", err)
		}
		return res
	case tui.RendererLive:
		return p.live(run, console)
	}
	return p.render(run)
}

// finishRun prints problems and maps the outcome to an exit code.
func finishRun(cmd *cobra.Command, i do.Injector, res engine.RunResult, reportErr error) error {
	g := do.MustInvoke[*GlobalOptions](i)
	if g.Output != OutputJSON {
		failures := slices.DeleteFunc(slices.Clone(res.Diags), func(d diag.Diagnostic) bool { return d.Severity != diag.SeverityError })
		if err := PrintDiagnostics(cmd.ErrOrStderr(), OutputText, do.MustInvoke[Env](i).Cwd, failures); err != nil {
			return err
		}
	}
	code := runExitCode(res)
	switch {
	case res.ReportFile == "":
		// Every run's report is saved (design §7.7); not saving it is a
		// system failure (exit 4), with the cause in res.Err.
		cmd.PrintErrf("error: the run report could not be saved: %v\n", res.Err)
		code = max(code, ExitSystem)
	case res.Err != nil:
		cmd.PrintErrf("warning: %v\n", res.Err)
	}
	if reportErr != nil {
		cmd.PrintErrf("error: %v\n", reportErr)
		code = max(code, ExitSystem)
	}
	if code != ExitOK {
		return Exit(code)
	}
	return nil
}

// runExitCode follows design §12: a failed node is 1, cancelled or aborted
// 3, a transport or system failure 4.
func runExitCode(res engine.RunResult) int {
	codes := map[engine.Status]int{
		engine.StatusPassed: ExitOK, engine.StatusFailed: ExitFailed, engine.StatusError: ExitSystem,
		engine.StatusCancelled: ExitCancelled, engine.StatusAborted: ExitCancelled,
	}
	if code, ok := codes[res.Status]; ok {
		return code
	}
	return ExitFailed
}

func writeFormat(w io.Writer, format string, res engine.RunResult) error {
	if format == harFormat {
		return engine.WriteHAR(w, res.Captured, buildinfo.Version())
	}
	return reportWriters()[format](w, res.Report)
}

// runMatrix runs the flow once for each combination and prints a grid. Cells run one after the other and
// print nothing of their own; the grid says which failed and why. The exit code is the worst of the cells.
func runMatrix(cmd *cobra.Command, s *Session, target flowi.FlowTarget, base flowi.RunOptions, reports []reportSpec, f runFlags) error {
	dims, err := parseMatrix(f.matrix)
	if err != nil {
		return err
	}
	if err := checkMatrixSize(dims); err != nil {
		return err
	}
	var results []cellResult
	worst := ExitOK
	for _, c := range matrixCells(dims) {
		r, code := runCell(cmd, s, target, c.options(base, dims), cellFiles{reports: reports, bundle: f.bundle, label: c.label()})
		r.cell = c
		results = append(results, r)
		worst = max(worst, code)
	}
	renderGrid(cmd.OutOrStdout(), dims, results)
	if worst != ExitOK {
		return Exit(worst)
	}
	return nil
}

// cellFiles are the report and bundle files a cell writes, named after the cell.
type cellFiles struct {
	reports []reportSpec
	bundle  string
	label   string
}

func runCell(cmd *cobra.Command, s *Session, target flowi.FlowTarget, opts flowi.RunOptions, files cellFiles) (cellResult, int) {
	run, err := s.Workspace.RunFlow(cmd.Context(), target, opts)
	if err != nil {
		return cellResult{outcome: outcomeError, why: errorLine(err)}, ExitCodeFor(cmd.Context(), err)
	}
	for range run.Events() { //nolint:revive // a cell prints nothing of its own: draining lets the run end
	}
	res := run.Wait()
	suffix := cellSuffix(files.label)
	specs := reportsFor(files.reports, suffix)
	if err := errors.Join(writeReports(specs, res), writeBundle(s, bundlePathFor(files.bundle, suffix, true), target, opts, res)); err != nil {
		return cellResult{outcome: outcomeError, why: err.Error()}, ExitSystem
	}
	return cellResult{outcome: outcomeOf(res), why: whyNotPassed(res)}, runExitCode(res)
}

// cellSuffix makes a cell's label safe to put in a file name.
func cellSuffix(label string) string {
	return strings.Trim(strings.NewReplacer(" · ", "-", "/", "-", " ", "-", "@", "-").Replace(label), "-")
}

func outcomeOf(res engine.RunResult) outcome {
	switch runExitCode(res) {
	case ExitOK:
		return outcomePassed
	case ExitFailed:
		return outcomeFailed
	case ExitCancelled:
		return outcomeStopped
	}
	return outcomeError
}

// whyNotPassed is the first thing that went wrong in a run: the failed step and its first message, or the
// run's own first error.
func whyNotPassed(res engine.RunResult) string {
	if res.Status == engine.StatusPassed {
		return ""
	}
	for _, d := range res.Diags {
		if d.Severity == diag.SeverityError {
			msg, _ := diag.English().Message(d)
			return string(d.Code) + " " + msg
		}
	}
	return "the run " + string(res.Status)
}

// errorLine is an error as one line, with the codes of the problems it holds.
func errorLine(err error) string {
	var list *diag.ListError
	if !errors.As(err, &list) {
		return err.Error()
	}
	parts := make([]string, 0, len(list.List))
	for _, d := range list.List {
		msg, _ := diag.English().Message(d)
		parts = append(parts, string(d.Code)+" "+msg)
	}
	return strings.Join(parts, "; ")
}
