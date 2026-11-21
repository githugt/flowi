// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/githugt/flowi/lib/schema"
)

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [kind]",
		Short: "Print the JSON Schema of a file kind (request, flow, environment, ...)",
		Long: "Prints the JSON Schema of one kind of Flowi file, for an editor or a CI step that validates YAML.\n" +
			"Without a kind it lists the kinds. To get completion in an editor with the YAML language server, put\n" +
			"the line printed by `flowi schema --modeline <kind>` at the top of the file.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modeline, _ := cmd.Flags().GetBool("modeline")
			if len(args) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(schema.Kinds(), "\n"))
				return err
			}
			b, ok := schema.Get(args[0])
			if !ok {
				return &UsageError{Err: fmt.Errorf("no schema for %q (kinds: %s)", args[0], strings.Join(schema.Kinds(), ", "))}
			}
			if modeline {
				_, err := fmt.Fprint(cmd.OutOrStdout(), schema.Modeline(args[0]))
				return err
			}
			_, err := cmd.OutOrStdout().Write(b)
			return err
		},
	}
	cmd.Flags().Bool("modeline", false, "print the YAML modeline that points an editor at the schema, instead of the schema")
	return cmd
}
