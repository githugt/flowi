// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

func newAuthCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Look at the OAuth tokens flowi keeps",
		Long: "A token that a person had to approve in a browser (authorization_code and device_code grants) is kept in the\n" +
			"workspace vault, encrypted like a secret, so a restart does not send you back to the browser. Tokens of the grants\n" +
			"that need nobody are asked for again each time and are not kept.",
	}
	cmd.AddCommand(newAuthTokensCommand(i), newAuthForgetCommand(i))
	return cmd
}

// tokenRow is one saved token as `auth tokens` shows it. It never holds the token.
type tokenRow struct {
	ID         string `json:"id"`
	For        string `json:"for"`
	Expires    string `json:"expires"`
	Expired    bool   `json:"expired"`
	HasRefresh bool   `json:"has_refresh"`
	Saved      bool   `json:"saved"`
}

func newAuthTokensCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "tokens",
		Short: "List the saved OAuth tokens and when each runs out",
		Long: "Lists the OAuth tokens kept in the vault: what each is for, when it expires, and whether it has a refresh token\n" +
			"(an expired token with one is renewed without asking you). The tokens themselves are never printed.\n" +
			"Opening the vault may ask for its passphrase.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			var rows []tokenRow
			for _, t := range s.Workspace.AuthTokens() {
				rows = append(rows, tokenRow{ID: t.ID, For: t.Label, Expires: t.Expires.Local().Format("2006-01-02 15:04 MST"),
					Expired: t.Expired, HasRefresh: t.HasRefresh, Saved: t.Stored})
			}
			if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
				return writeJSONLines(cmd.OutOrStdout(), rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No saved tokens. One is kept after you approve a sign-in (authorization_code or device_code).")
				return nil
			}
			return writeTokenTable(cmd.OutOrStdout(), rows)
		},
	}
}

func writeTokenTable(w io.Writer, rows []tokenRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tEXPIRES\tREFRESH\tFOR")
	for _, r := range rows {
		expires := r.Expires
		if r.Expired {
			expires += " (expired)"
		}
		refresh := "no"
		if r.HasRefresh {
			refresh = "yes"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, expires, refresh, r.For)
	}
	return tw.Flush()
}

func newAuthForgetCommand(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:     "forget <id>",
		Short:   "Drop a saved OAuth token, so the next send signs in again",
		Long:    "Drops the token whose id (or the start of it) `flowi auth tokens` shows, from the vault. The next send that needs it asks you to sign in again.",
		Example: "  flowi auth forget 3fa9c1d2",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			if err := s.Workspace.ForgetAuthToken(args[0]); err != nil {
				return err
			}
			cmd.Printf("Forgot token %s\n", args[0])
			return nil
		},
	}
}
