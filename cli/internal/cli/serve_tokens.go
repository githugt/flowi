// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

func newServeTokensCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Make, list, rotate and revoke named trigger tokens",
		Long: "Named trigger tokens are what callers of `flowi serve` send as `Authorization: Bearer <token>`. Each one can be\n" +
			"limited to some flows and environments and can expire, so a CI job gets a token that starts only what it needs,\n" +
			"and a leaked one is revoked on its own. Only a hash of the value is kept: it is printed once, when the token is\n" +
			"made or rotated. A running `flowi serve` sees a token made, rotated or revoked at once. The single token of\n" +
			"earlier releases (`flowi serve token`) keeps working and is listed as \"default\".",
	}
	cmd.AddCommand(newServeTokensNewCommand(i), newServeTokensListCommand(i), newServeTokensRotateCommand(i), newServeTokensRemoveCommand(i))
	return cmd
}

func newServeTokensNewCommand(i do.Injector) *cobra.Command {
	var flows, envs []string
	var expires string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Make a named token, optionally limited to flows, environments and a time",
		Long: "Makes a token and prints it once on standard output. Without --flow and --env it may start any flow in any\n" +
			"environment; with --env, a call that names no environment is refused.\n\n" +
			"  --flow smoke/*     a flow path; * matches any characters, so smoke/* is every flow under smoke/\n" +
			"  --env staging      an environment name\n" +
			"  --expires 30d      a time such as 90m, 12h, 30d or 2w; without it the token never expires\n\n" +
			"Repeat --flow and --env to allow several.",
		Example: "  flowi serve tokens new ci --flow 'smoke/*' --env staging --expires 30d",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lifetime, err := parseLifetime(expires)
			if err != nil {
				return &UsageError{Err: err}
			}
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			secret, info, err := s.Workspace.NewScopedTriggerToken(args[0], flows, envs, lifetime)
			if err != nil {
				return err
			}
			printNewToken(cmd, secret, info)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&flows, "flow", nil, "a flow path the token may start; * matches any characters (repeatable; default any flow)")
	cmd.Flags().StringArrayVar(&envs, "env", nil, "an environment the token may use (repeatable; default any environment)")
	cmd.Flags().StringVar(&expires, "expires", "", "how long the token lasts, such as 12h or 30d (default: never)")
	return cmd
}

func newServeTokensListCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List the trigger tokens with their limits (never their values)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			list, err := s.Workspace.TriggerTokens()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No trigger tokens. Make one with: flowi serve tokens new <name>")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tID\tFLOWS\tENVS\tEXPIRES")
			for _, t := range list {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.Name, dash(t.ID), anyOr(t.Flows), anyOr(t.Envs), expiryText(t))
			}
			return tw.Flush()
		},
	}
}

func newServeTokensRotateCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "rotate <name>",
		Short: "Give a token a new value; the old one stops working at once",
		Long: "Replaces the value of a token (by name or id) and prints the new one once. Its limits stay; a lifetime starts\n" +
			"again from now. `default` replaces the single token of earlier releases.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			secret, info, err := s.Workspace.RotateTriggerToken(args[0])
			if err != nil {
				return err
			}
			printNewToken(cmd, secret, info)
			return nil
		},
	}
}

func newServeTokensRemoveCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"revoke", "remove"},
		Short:   "Revoke a token; it stops working at once, also in a running `flowi serve`",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			if err := s.Workspace.RevokeTriggerToken(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Revoked %s.\n", args[0])
			return nil
		},
	}
}

func printNewToken(cmd *cobra.Command, secret string, info flowi.TriggerTokenInfo) {
	fmt.Fprintln(cmd.OutOrStdout(), secret)
	msg := "Keep this token: only a hash is stored, so it is not shown again."
	if info.Legacy {
		msg = "Keep this token: it is stored encrypted and not shown again."
	}
	fmt.Fprintln(cmd.ErrOrStderr(), msg)
}

// parseLifetime reads --expires: a Go duration (90m, 12h) or whole days and weeks (30d, 2w). Empty is no expiry.
func parseLifetime(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	unit := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour}[s[len(s)-1]]
	if unit > 0 {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil || n < 1 || n > 3650 {
			return 0, fmt.Errorf("--expires %q: use a whole number of days or weeks, such as 30d or 2w", s)
		}
		return time.Duration(n) * unit, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < time.Minute {
		return 0, fmt.Errorf("--expires %q: use a time of at least a minute, such as 90m, 12h, 30d or 2w", s)
	}
	return d, nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func anyOr(list []string) string {
	if len(list) == 0 {
		return "any"
	}
	return strings.Join(list, ",")
}

func expiryText(t flowi.TriggerTokenInfo) string {
	switch {
	case t.Expires.IsZero():
		return "never"
	case t.Expired:
		return "expired " + t.Expires.Local().Format("2006-01-02 15:04")
	default:
		return t.Expires.Local().Format("2006-01-02 15:04")
	}
}
