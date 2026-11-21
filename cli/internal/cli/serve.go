// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/serve/replay"
	"github.com/githugt/flowi/lib/vault"
)

type serveFlags struct {
	replayAddr, triggerAddr, healthAddr, record    string
	recordTo, recordFolder, recordHAR, passThrough string
	forward, forwardTo, forwardEnv                 string
	forwardOnly                                    []string
	valkeyAddr, tlsCert, tlsKey                    string
	latency, shutdown                              time.Duration
	faultError, faultDrop                          float64
	faultStatus                                    int
	faultJitter                                    time.Duration
	sequence, cors, quiet, metrics                 bool
	noReplay, noTriggers, noSchedules              bool
	insecureListen                                 bool
}

func newServeCommand(i do.Injector) *cobra.Command {
	var f serveFlags
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Replay saved responses, run flows on HTTP calls and on schedules",
		Long: "Run a long-lived process with up to three parts:\n\n" +
			"  Replayer   answers HTTP calls with the pinned examples and saved responses of your requests\n" +
			"             (default 127.0.0.1:8525). Steer it with request headers: X-Flowi-Status, X-Flowi-Example,\n" +
			"             X-Flowi-Request, X-Flowi-Version, X-Flowi-Latency, X-Flowi-Fault (none, drop, error or error:<status>). GET /__flowi/routes lists what it knows.\n" +
			"  Triggers   POST /flows/<flow>/run starts a flow (default 127.0.0.1:8526). Needs a bearer token:\n" +
			"             make a named one, limited to flows, environments and a time, with `flowi serve tokens new`, or the single\n" +
			"             full-power one with `flowi serve token`, or set FLOWI_SECRET_SERVE_TRIGGER_TOKEN. Plain HTTP: on an address\n" +
			"             other machines can reach it needs --tls-cert and --tls-key, or --insecure-listen behind a TLS proxy.\n" +
			"  Schedules  runs the cron schedules of workspace.yaml (see `flowi schedule`).\n\n" +
			"With --health-addr it also answers GET /healthz (alive) and GET /readyz (200, or 503 while it drains or every run slot\n" +
			"is taken) with no token, and with --metrics GET /metrics, for an orchestrator or a monitor.\n\n" +
			"All listeners bind to localhost unless you say otherwise. Ctrl-C (or SIGTERM) stops accepting calls,\n" +
			"lets running requests and flows finish for up to --shutdown-timeout, then cancels what is left.\n" +
			"Exit codes: 0 stopped cleanly, 3 work had to be cancelled at the timeout, 2 bad options, 4 a system failure.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runServe(cmd, i, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.replayAddr, "replay-addr", "", "replayer address (default: the serve.replay_addr setting)")
	fl.StringVar(&f.triggerAddr, "trigger-addr", "", "trigger address (default: the serve.trigger_addr setting)")
	fl.StringVar(&f.tlsCert, "tls-cert", "", "PEM certificate chain the trigger endpoint serves HTTPS with (with --tls-key)")
	fl.StringVar(&f.tlsKey, "tls-key", "", "PEM private key for --tls-cert")
	fl.BoolVar(&f.insecureListen, "insecure-listen", false, "allow the trigger endpoint on a network-reachable address without TLS, when a proxy in front terminates TLS")
	fl.StringVar(&f.healthAddr, "health-addr", "", "serve /healthz and /readyz (no token) on this address, host:port, for an orchestrator's probes; off by default")
	fl.BoolVar(&f.metrics, "metrics", false, "with --health-addr: also serve /metrics in the Prometheus text format")
	fl.DurationVar(&f.latency, "latency", 0, "delay added to every replayed answer (default: the serve.latency setting)")
	fl.Float64Var(&f.faultError, "fault-error", 0, "share of replayed calls (0 to 1) answered with an error on purpose, to try retries and error handling")
	fl.IntVar(&f.faultStatus, "fault-status", 0, "status of an injected error (default 503)")
	fl.Float64Var(&f.faultDrop, "fault-drop", 0, "share of replayed calls (0 to 1) whose connection is closed with no answer")
	fl.DurationVar(&f.faultJitter, "fault-jitter", 0, "up to this much extra random delay on every replayed answer")
	fl.BoolVar(&f.sequence, "sequence", false, "answer a request's responses one call at a time, in file-name order, then start over")
	fl.BoolVar(&f.cors, "cors", false, "let browsers call the replayer from any origin")
	fl.StringVar(&f.record, "record", "", "forward calls to this real API and save its responses; a call no request matches becomes a new request")
	fl.StringVar(&f.passThrough, "pass-through", "", "answer calls from saved responses, and forward the calls no saved response can answer to this real API (nothing is saved)")
	fl.StringVar(&f.forward, "forward", "", "what to do with a call no saved response can answer: pass (send it to a real API and relay the answer) or save (also save the answer, so the next such call is replayed); calls a saved response can answer are always replayed")
	fl.StringVar(&f.forwardTo, "forward-to", "", "with --forward: one real API (base URL) that takes every such call; without it a call goes to the API of the saved request it matches")
	fl.StringSliceVar(&f.forwardOnly, "forward-only", nil, "with --forward and no --forward-to: forward only to these APIs (base URLs, comma separated); the default is the API of any saved request")
	fl.StringVar(&f.forwardEnv, "forward-env", "", "with --forward: the environment the saved requests' base URLs are worked out in (default: the defaults.environment setting)")
	fl.StringVar(&f.recordFolder, "record-folder", "", "with --record-to: the folder of that collection's requests new requests are saved in, like api/recorded (default: recorded)")
	fl.StringVar(&f.recordTo, "record-to", "", "with --record or --forward save: the collection (name or id) new requests are saved in (default: the Scratch draft)")
	fl.StringVar(&f.recordHAR, "record-har", "", "also write the calls the replayer handles (replayed, forwarded or recorded) to this HAR file, in order (credentials masked); `flowi import har` turns it into a flow whose steps read the values earlier responses gave")
	fl.BoolVar(&f.noReplay, "no-replay", false, "do not start the replayer")
	fl.BoolVar(&f.noTriggers, "no-triggers", false, "do not start the HTTP triggers")
	fl.BoolVar(&f.noSchedules, "no-schedules", false, "do not run schedules")
	fl.DurationVar(&f.shutdown, "shutdown-timeout", 0, "how long to let running work finish at shutdown (default: the serve.shutdown_timeout setting)")
	fl.StringVar(&f.valkeyAddr, "valkey-addr", "", "share run records and schedule ticks with other instances through this Valkey/Redis server, host:port (default: the serve.valkey_addr setting)")
	fl.BoolVar(&f.quiet, "quiet", false, "do not print each replayed call")
	cmd.AddCommand(newServeTokenCommand(i), newServeTokensCommand(i), newServeValkeyPasswordCommand(i))
	return cmd
}

// options applies the flags the user gave on top of the settings.
func (f serveFlags) options(cmd *cobra.Command, base flowi.ServeOptions) flowi.ServeOptions {
	changed := cmd.Flags().Changed
	if changed("replay-addr") {
		base.ReplayAddr = f.replayAddr
	}
	if changed("trigger-addr") {
		base.TriggerAddr = f.triggerAddr
	}
	if changed("latency") {
		base.Latency = f.latency
	}
	if changed("valkey-addr") {
		base.ValkeyAddr = f.valkeyAddr
	}
	if changed("shutdown-timeout") {
		base.ShutdownTimeout = f.shutdown
	}
	base.HealthAddr, base.Metrics = f.healthAddr, f.metrics
	base.Faults = replay.Faults{ErrorRate: f.faultError, ErrorStatus: f.faultStatus, DropRate: f.faultDrop, Jitter: f.faultJitter}
	base.Sequence, base.CORS, base.Record, base.RecordTo, base.RecordHAR = f.sequence, f.cors, f.record, f.recordTo, f.recordHAR
	base.RecordFolder = f.recordFolder
	base.PassThrough = f.passThrough
	base.Forward, base.ForwardTo, base.ForwardOnly, base.ForwardEnv = f.forward, f.forwardTo, f.forwardOnly, f.forwardEnv
	base.NoReplay, base.NoTriggers, base.NoSchedules = f.noReplay, f.noTriggers, f.noSchedules
	base.TLSCert, base.TLSKey, base.InsecureListen = f.tlsCert, f.tlsKey, f.insecureListen
	return base
}

func runServe(cmd *cobra.Command, i do.Injector, f serveFlags) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	opts := f.options(cmd, s.Workspace.ServeDefaults())
	if logger, err := do.InvokeNamed[*slog.Logger](i, LoggerName); err == nil {
		opts.Logger = logger
	}
	srv, err := s.Workspace.Serve(cmd.Context(), opts)
	if err != nil {
		return asServeError(err)
	}
	out := cmd.OutOrStdout()
	printServing(out, srv, s.Workspace)
	if !f.quiet && srv.ReplayAddr() != "" {
		go printTraffic(out, srv.Traffic().Subscribe(cmd.Context()))
	}
	return finishServe(cmd, srv.Wait())
}

// asServeError makes unusable options a usage error (exit 2).
func asServeError(err error) error {
	if errors.Is(err, flowi.ErrInvalidServeOptions) {
		return &UsageError{Err: err}
	}
	return err
}

// finishServe reports how the server ended. A signal is the normal way to
// stop it, so a clean drain is success.
func finishServe(cmd *cobra.Command, err error) error {
	switch {
	case err == nil:
		fmt.Fprintln(cmd.OutOrStdout(), "Stopped.")
		return &StoppedError{}
	case errors.Is(err, flowi.ErrShutdownTimeout):
		fmt.Fprintf(cmd.ErrOrStderr(), "Stopped, but work was still going at the shutdown timeout and was cancelled: %v\n", err)
		return Exit(ExitCancelled)
	}
	return err
}

func printServing(w io.Writer, srv *flowi.Server, ws *flowi.Workspace) {
	if a := srv.ReplayAddr(); a != "" {
		fmt.Fprintf(w, "Replayer   http://%s\n", a)
	}
	if a := srv.TriggerAddr(); a != "" {
		scheme := "http"
		if srv.TriggerTLS() {
			scheme = "https"
		}
		fmt.Fprintf(w, "Triggers   %s://%s  (bearer token required)\n", scheme, a)
	}
	if a := srv.HealthAddr(); a != "" {
		fmt.Fprintf(w, "Health     http://%s/healthz  http://%s/readyz\n", a, a)
	}
	for _, u := range srv.Upcoming() {
		fmt.Fprintf(w, "Schedule   %s  next %s\n", u.Name, u.At.Format("2006-01-02 15:04 MST"))
	}
	for _, n := range srv.Notes() {
		fmt.Fprintf(w, "note: %s\n", n)
	}
	fmt.Fprintf(w, "Serving %d collection(s). Press Ctrl-C to stop.\n", len(ws.Collections()))
}

// printTraffic prints each replayed call until the channel closes.
func printTraffic(w io.Writer, entries <-chan replay.Entry) {
	for e := range entries {
		what := e.Request
		if what == "" {
			what = "-"
		}
		extra := e.Match
		if e.Example != "" {
			extra += " " + e.Example
		}
		fmt.Fprintf(w, "%s  %s %s -> %d  %s  [%s]\n", e.At.Format("15:04:05"), e.Method, e.Path, e.Status, what, extra)
	}
}

func newServeTokenCommand(i do.Injector) *cobra.Command {
	var rotate bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Create the bearer token that authorises HTTP triggers",
		Long: "Create the token callers send as `Authorization: Bearer <token>` to POST /flows/<flow>/run. It is stored\n" +
			"in the workspace vault and printed once, on standard output. Run it again with --rotate to replace it.\n" +
			"FLOWI_SECRET_SERVE_TRIGGER_TOKEN overrides the stored token (for containers).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runServeToken(cmd, i, rotate) },
	}
	cmd.Flags().BoolVar(&rotate, "rotate", false, "replace the existing token (the old one stops working)")
	return cmd
}

func runServeToken(cmd *cobra.Command, i do.Injector, rotate bool) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	_, exists, err := s.Workspace.TriggerToken()
	if err != nil {
		return err
	}
	if exists && !rotate {
		fmt.Fprintln(cmd.ErrOrStderr(), "A trigger token is already set; its value is not shown. Use --rotate to replace it.")
		return nil
	}
	token, err := s.Workspace.NewTriggerToken()
	if err != nil {
		return err
	}
	env := do.MustInvoke[Env](i)
	if _, overridden := vault.Overridden(env.Environ, "serve", "trigger_token"); overridden {
		fmt.Fprintln(cmd.ErrOrStderr(), "Note: FLOWI_SECRET_SERVE_TRIGGER_TOKEN is set and overrides this token.")
	}
	fmt.Fprintln(cmd.OutOrStdout(), token)
	fmt.Fprintln(cmd.ErrOrStderr(), "Keep this token: it is stored encrypted and not shown again.")
	return nil
}

func newServeValkeyPasswordCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "valkey-password",
		Short: "Store the password of the shared Valkey/Redis server (read from standard input)",
		Long: "Reads one line from standard input and stores it encrypted in the workspace vault, for serve.valkey_addr.\n" +
			"For example: printf '%s' \"$PASSWORD\" | flowi serve valkey-password.\n" +
			"FLOWI_SECRET_SERVE_VALKEY_PASSWORD overrides the stored value (for containers).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			line, err := bufio.NewReader(io.LimitReader(cmd.InOrStdin(), maxPasswordBytes)).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("reading the password: %w", err)
			}
			password := strings.TrimRight(line, "\r\n")
			if password == "" {
				return &UsageError{Err: errors.New("no password on standard input")}
			}
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			if err := s.Workspace.SetValkeyPassword(password); err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "The Valkey password is stored in the workspace vault.")
			return nil
		},
	}
}

// maxPasswordBytes bounds what is read from standard input.
const maxPasswordBytes = 4096
