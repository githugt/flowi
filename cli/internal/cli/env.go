// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

func newEnvCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments (global to the workspace)",
		Long: "Environments belong to the workspace, not to a collection: every collection sees the same set. They live\n" +
			"in the workspace's environments directory and are not committed with a collection.\n\n" +
			"A collection can also carry its own environments folder (`flowi env share` writes one). Where the workspace\n" +
			"has no environment of that name, a send or run uses the collection's file, so a clone of a repository works\n" +
			"at once; its secrets come from FLOWI_SECRET_<ENV>_<NAME>. `flowi env import` copies them into the workspace,\n" +
			"where secrets can be stored in the vault.",
	}
	cmd.AddCommand(newEnvLs(i), newEnvNew(i), newEnvSet(i), newEnvUnset(i), newEnvImport(i), newEnvShare(i))
	return cmd
}

func newEnvLs(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List the workspace's environments, and the ones collections ship that it lacks",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return runEnvLs(cmd, i) },
	}
}

func newEnvNew(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create an empty environment in the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			file, err := s.Workspace.NewEnvironment(args[0])
			if err != nil {
				return err
			}
			cmd.Printf("Created environment %s (%s)\n", args[0], file)
			return nil
		},
	}
}

func newEnvSet(i do.Injector) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "set <env> <name> <value>",
		Short: "Set a plain variable of an environment (a secret's value is set with `flowi secret set`)",
		Long: "Adds the variable, or replaces its value. --type says what the value is read as: string (the default),\n" +
			"number, boolean or json, so {\"n\": {{env.retries}}} sends a number and not \"3\". A secret variable is not\n" +
			"set here: its value belongs in the vault.",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := parseVarKind(kind)
			if err != nil {
				return err
			}
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			added, err := s.Workspace.SetEnvironmentVar(args[0], args[1], args[2], k)
			if err != nil {
				return refusedAsUsage(err)
			}
			verb := "Set"
			if added {
				verb = "Added"
			}
			cmd.Printf("%s %s in %s\n", verb, args[1], args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "type", "string", "what the value is: string, number, boolean or json")
	return cmd
}

func newEnvUnset(i do.Injector) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <env> <name>",
		Short: "Remove a plain variable from an environment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			removed, err := s.Workspace.UnsetEnvironmentVar(args[0], args[1])
			if err != nil {
				return refusedAsUsage(err)
			}
			if !removed {
				cmd.Printf("%s has no variable %s\n", args[0], args[1])
				return nil
			}
			cmd.Printf("Removed %s from %s\n", args[1], args[0])
			return nil
		},
	}
}

func newEnvImport(i do.Injector) *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "import [name...]",
		Short: "Copy the environments a collection ships into the workspace",
		Long: "Copies the environment files of a collection's own environments folder that the workspace does not have\n" +
			"(all of them, or the names given). A secret comes over as a declaration with no value: set it with\n" +
			"`flowi secret set <env> <name>`. An environment the workspace already has is never touched.",
		RunE: func(cmd *cobra.Command, names []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			id, err := s.Workspace.FindCollection(collection)
			if err != nil {
				return &UsageError{Err: err}
			}
			added, err := s.Workspace.ImportEnvironments(id, names...)
			if err != nil {
				return err
			}
			if len(added) == 0 {
				cmd.Println("Nothing to import: the workspace already has every environment the collection ships.")
				return nil
			}
			cmd.Printf("Imported %s\n", strings.Join(added, ", "))
			return nil
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "collection (name or id) to import from (default: the only one)")
	return cmd
}

func newEnvShare(i do.Injector) *cobra.Command {
	var collection string
	var force bool
	cmd := &cobra.Command{
		Use:   "share <env>",
		Short: "Write an environment into a collection's own folder, so it goes with the collection in git",
		Long: "Writes <collection>/environments/<env>.yaml with the variables, their kinds, the colour and the base. A\n" +
			"secret is written as a declaration with no value: values stay in the vault, so a colleague or CI supplies\n" +
			"their own (FLOWI_SECRET_<ENV>_<NAME>). A file already there is kept unless --force is given.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			id, err := s.Workspace.FindCollection(collection)
			if err != nil {
				return &UsageError{Err: err}
			}
			file, existed, notes, err := s.Workspace.ShareEnvironment(id, args[0], force)
			if err != nil {
				return err
			}
			if existed {
				return &UsageError{Err: fmt.Errorf("%s already exists: --force replaces it", file)}
			}
			for _, n := range notes {
				cmd.PrintErrln("note:", n)
			}
			cmd.Printf("Wrote %s\n", file)
			return nil
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "collection (name or id) to share with (default: the only one)")
	cmd.Flags().BoolVar(&force, "force", false, "replace the file if the collection already has one")
	return cmd
}

// refusedAsUsage makes a request the library refused as a mistake of the caller (exit 2); other errors stay as they are.
func refusedAsUsage(err error) error {
	if errors.Is(err, flowi.ErrEnvironmentVar) {
		return &UsageError{Err: err}
	}
	return err
}

// parseVarKind reads --type; anything else is a usage error that lists the choices.
func parseVarKind(s string) (model.VarKind, error) {
	switch s {
	case "", "string":
		return model.VarString, nil
	case "number":
		return model.VarNumber, nil
	case "boolean":
		return model.VarBoolean, nil
	case "json":
		return model.VarJSON, nil
	}
	return "", &UsageError{Err: errors.New("--type is string, number, boolean or json, got " + fmt.Sprintf("%q", s))}
}

// envRow is one line of `env ls`.
type envRow struct {
	Name   string `json:"name"`
	Vars   int    `json:"vars"`
	Source string `json:"source"` // "workspace", or "collection <name>" for one that is only shipped
}

func runEnvLs(cmd *cobra.Command, i do.Injector) error {
	s, err := session(cmd, i)
	if err != nil {
		return err
	}
	var rows []envRow
	for _, e := range s.Workspace.Environments() {
		rows = append(rows, envRow{Name: e.Name, Vars: len(e.Vars), Source: "workspace"})
	}
	for _, sh := range s.Workspace.ShippedEnvironments() {
		rows = append(rows, envRow{Name: sh.Environment.Name, Vars: len(sh.Environment.Vars), Source: "collection " + sh.Name})
	}
	if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
		return writeJSONLines(cmd.OutOrStdout(), rows)
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tVARIABLES\tFROM")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\n", r.Name, r.Vars, r.Source)
	}
	return tw.Flush()
}
