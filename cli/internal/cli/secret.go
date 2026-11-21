// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"text/tabwriter"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

func newSecretCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secret values in the workspace vault",
		Long: "Secret variables are declared in environment files as { secret: true }; their values live only in the\n" +
			"encrypted workspace vault. FLOWI_SECRET_<ENV>_<NAME> environment variables override the vault (for CI).",
	}
	var env string
	set := &cobra.Command{
		Use:   "set <env> <name>",
		Short: "Store a secret (the value is read from the terminal without echo, or from standard input)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretSet(cmd, i, args[0], args[1])
		},
	}
	unset := &cobra.Command{
		Use:   "unset <env> <name>",
		Short: "Remove a secret from the vault",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretUnset(cmd, i, args[0], args[1])
		},
	}
	ls := &cobra.Command{
		Use:   "ls",
		Short: "List secret variables and whether each is set (values are never shown)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runSecretLs(cmd, i, env, "") },
	}
	ls.Flags().StringVar(&env, "env", "", "only this environment")
	var reveal bool
	get := &cobra.Command{
		Use:   "get <env> <name>",
		Short: "Say whether a secret is set; with --reveal, print its value",
		Long: "Without --reveal this prints only the secret's state. With --reveal the value is printed on its own,\n" +
			"so $(flowi secret get local password --reveal) works in a script. A secret that FLOWI_SECRET_<ENV>_<NAME>\n" +
			"provides is not revealed: the variable is already in your environment.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretGet(cmd, i, args[0], args[1], reveal)
		},
	}
	get.Flags().BoolVar(&reveal, "reveal", false, "print the secret's value")
	var checkEnv string
	check := &cobra.Command{
		Use:   "check",
		Short: "Fail (exit 2) when a declared secret has no value, so CI stops before a run is half done",
		Long: "Lists the secrets the environments declare that have neither a vault value nor a FLOWI_SECRET_<ENV>_<NAME>\n" +
			"override. Nothing is printed and the exit code is 0 when all are set. Use it as the first step of a CI job:\n" +
			"  flowi secret check --env staging",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runSecretCheck(cmd, i, checkEnv) },
	}
	check.Flags().StringVar(&checkEnv, "env", "", "only this environment")
	var dryRun bool
	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove stored secrets that no environment declares any more",
		Long: "Deleting an environment removes its secrets, but a secret whose variable was removed or made plain, or an\n" +
			"environment deleted by another tool, can leave a value in the vault with nothing to show it. This lists those\n" +
			"and removes them. A secret an environment still declares is never touched. --dry-run only lists.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runSecretPrune(cmd, i, dryRun) },
	}
	prune.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be removed and change nothing")
	cmd.AddCommand(set, unset, ls, get, check, prune, newSecretHostsCommand(i), newSecretAllowHostCommand(i), newSecretDenyHostCommand(i))
	return cmd
}

func runSecretSet(cmd *cobra.Command, i do.Injector, env, name string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	if err := s.Workspace.CheckSecret(env, name); err != nil {
		return err // before prompting: do not ask for a value that cannot be stored
	}
	value, err := do.MustInvoke[Console](i).ReadSecret(fmt.Sprintf("Value for %s/%s: ", env, name))
	if errors.Is(err, errNoInput) || (err == nil && value == "") {
		return &UsageError{Err: errors.New("no value given: type it at the prompt, or pipe it on standard input")}
	}
	if err != nil {
		return err
	}
	if err := s.Workspace.SetSecret(env, name, value); err != nil {
		return err
	}
	cmd.Printf("Set secret %s for %s\n", name, env)
	return nil
}

// runSecretPrune removes (or with dryRun only lists) the stored secrets nothing declares any more.
func runSecretPrune(cmd *cobra.Command, i do.Injector, dryRun bool) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	var found []flowi.OrphanSecret
	if dryRun {
		found, err = s.Workspace.OrphanSecrets()
	} else {
		found, err = s.Workspace.PruneSecrets()
	}
	if err != nil {
		return err
	}
	rows := orphanRows(found, dryRun)
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), rows)
	}
	if len(rows) == 0 {
		cmd.Println("No orphaned secrets.")
		return nil
	}
	if err := writeSecretTable(cmd.OutOrStdout(), rows); err != nil {
		return err
	}
	if dryRun {
		cmd.Printf("%s would be removed; run without --dry-run to remove them\n", plural(len(rows), "secret"))
	} else {
		cmd.Printf("Removed %s\n", plural(len(rows), "secret"))
	}
	return nil
}

// orphanRows words the orphans as table rows: the state says whether they were removed, would be, and why they are orphans.
func orphanRows(found []flowi.OrphanSecret, dryRun bool) []secretRow {
	rows := make([]secretRow, 0, len(found))
	for _, o := range found {
		state := "removed"
		if dryRun {
			state = "orphan"
		}
		if o.EnvironmentGone {
			state += " (environment gone)"
		}
		rows = append(rows, secretRow{Environment: o.Environment, Name: o.Name, State: state})
	}
	return rows
}

func runSecretUnset(cmd *cobra.Command, i do.Injector, env, name string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	if err := s.Workspace.UnsetSecret(env, name); err != nil {
		return err
	}
	cmd.Printf("Removed secret %s for %s\n", name, env)
	return nil
}

func runSecretGet(cmd *cobra.Command, i do.Injector, env, name string, reveal bool) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	if !reveal {
		return runSecretStatus(cmd, i, s, env, name)
	}
	value, err := s.Workspace.RevealSecret(env, name)
	if err != nil {
		return err
	}
	slog.Info("a secret was revealed", "Event", "SecretRevealed", "Environment", env, "Name", name)
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), []secretValue{{Environment: env, Name: name, Value: value}})
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
	return err
}

// runSecretStatus says whether one secret is set. A name the environment does not declare as a
// secret is an error: an empty table would look like "not set".
func runSecretStatus(cmd *cobra.Command, i do.Injector, s *Session, env, name string) error {
	infos, err := s.Workspace.Secrets()
	if err != nil {
		return err
	}
	rows := secretRows(infos, env, name)
	if len(rows) == 0 {
		return &UsageError{Err: fmt.Errorf("no secret %s in environment %s", name, env)}
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), rows)
	}
	return writeSecretTable(cmd.OutOrStdout(), rows)
}

type secretValue struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

type secretRow struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	State       string `json:"state"` // set | missing | env override
}

func runSecretLs(cmd *cobra.Command, i do.Injector, env, name string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	infos, err := s.Workspace.Secrets()
	if err != nil {
		return err
	}
	rows := secretRows(infos, env, name)
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), rows)
	}
	return writeSecretTable(cmd.OutOrStdout(), rows)
}

// runSecretCheck is the CI preflight: it names every declared secret without a value and exits 2 if there is one.
// Before: a missing secret failed in the middle of a run, after earlier steps had already sent requests.
func runSecretCheck(cmd *cobra.Command, i do.Injector, env string) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	infos, err := s.Workspace.Secrets()
	if err != nil {
		return err
	}
	missing := missingSecrets(secretRows(infos, env, ""))
	if len(missing) == 0 {
		return nil
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		if err := writeJSONLines(cmd.OutOrStdout(), missing); err != nil {
			return err
		}
	} else if err := writeSecretTable(cmd.OutOrStdout(), missing); err != nil {
		return err
	}
	cmd.PrintErrf("%d secret(s) have no value: set them with flowi secret set, or FLOWI_SECRET_<ENV>_<NAME>\n", len(missing))
	return Exit(ExitUsage)
}

// missingSecrets keeps the rows that have no value.
func missingSecrets(rows []secretRow) []secretRow {
	var out []secretRow
	for _, r := range rows {
		if r.State == "missing" {
			out = append(out, r)
		}
	}
	return out
}

func secretRows(infos []flowi.SecretInfo, env, name string) []secretRow {
	var rows []secretRow
	for _, s := range infos {
		if (env != "" && s.Environment != env) || (name != "" && s.Name != name) {
			continue
		}
		state := "missing"
		switch {
		case s.Override:
			state = "env override"
		case s.Set:
			state = "set"
		}
		rows = append(rows, secretRow{Environment: s.Environment, Name: s.Name, State: state})
	}
	return rows
}

func writeSecretTable(w io.Writer, rows []secretRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ENV\tNAME\tSTATE")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Environment, r.Name, r.State)
	}
	return tw.Flush()
}

// writeJSONLines writes one JSON object per row.
func writeJSONLines[T any](w io.Writer, rows []T) error {
	enc := json.NewEncoder(w)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	}
	return nil
}

func newSecretHostsCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "hosts",
		Short: "List the hosts you allowed to receive secrets",
		Long: "A request that would send a secret to a host is stopped (FLW-SEC-005) unless the host is named by a variable of\n" +
			"the selected environment, is this computer, or is allowed here. These are the hosts you allowed with\n" +
			"`flowi secret allow-host`; a collection file cannot add to them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			hosts := s.Workspace.AllowedSecretHosts()
			if len(hosts) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No hosts allowed. Secrets go only to the hosts the selected environment names, and to this computer.")
				return nil
			}
			for _, h := range hosts {
				fmt.Fprintln(cmd.OutOrStdout(), h)
			}
			return nil
		},
	}
}

func newSecretAllowHostCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "allow-host <host>",
		Short: "Let secrets be sent to a host the selected environment does not name",
		Long: "Adds a host to your own list of hosts that may receive secrets (your user settings: a collection cannot add one).\n" +
			"  api.example.com        that host on any port\n" +
			"  api.example.com:8443   that host on that port only\n" +
			"  *.example.com          any subdomain of example.com\n" +
			"An address such as https://api.example.com/v1 is cut down to its host. Allowing a host twice changes nothing.",
		Example: "  flowi secret allow-host api.example.com",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			host, err := s.Workspace.AllowSecretHost(args[0])
			if err != nil {
				return &UsageError{Err: err}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Secrets may now be sent to %s.\n", host)
			return nil
		},
	}
}

func newSecretDenyHostCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "deny-host <host>",
		Short: "Take a host off the list of hosts allowed to receive secrets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			removed, err := s.Workspace.DenySecretHost(args[0])
			if err != nil {
				return &UsageError{Err: err}
			}
			if !removed {
				return &UsageError{Err: fmt.Errorf("%s is not on the list; see `flowi secret hosts`", args[0])}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Secrets are no longer sent to %s unless an environment names it.\n", args[0])
			return nil
		},
	}
}
