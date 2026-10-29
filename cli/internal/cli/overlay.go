// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"
)

func newOverlayCommand(i do.Injector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overlay",
		Short: "List, show and delete flowi overlays (a request as a VIP customer, a slow request, ...)",
		Long: "A flowi overlay is a named set of changes laid over a request version when it is sent:\n" +
			"flowi send <request> --flowi-overlay vip. It is not an OpenAPI Overlay (flowi import --overlay),\n" +
			"which fixes a description before it is imported.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newOverlayLsCommand(i), newOverlayShowCommand(i), newOverlayExportCommand(i), newOverlayRmCommand(i))
	return cmd
}

// jsonOverlay is the stable JSON shape of a flowi overlay.
type jsonOverlay struct {
	Collection  string   `json:"collection"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Shared      int      `json:"shared_changes"`
	Requests    []string `json:"requests"`
	File        string   `json:"file"`
}

func jsonOverlayOf(c model.Collection, o model.Overlay) jsonOverlay {
	out := jsonOverlay{Collection: c.Manifest.Name, Name: o.Name, Description: o.Description, Shared: len(o.Shared), Requests: []string{}, File: o.File}
	for _, r := range o.Requests {
		out.Requests = append(out.Requests, r.Path)
	}
	return out
}

func newOverlayLsCommand(i do.Injector) *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the flowi overlays of mounted collections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			found := []jsonOverlay{}
			for _, c := range s.Workspace.Collections() {
				if collection != "" && c.Manifest.Name != collection && string(c.Manifest.ID) != collection {
					continue
				}
				for _, o := range c.Overlays {
					found = append(found, jsonOverlayOf(c, o))
				}
			}
			if do.MustInvoke[*GlobalOptions](i).Output == OutputJSON {
				return writeJSON(cmd.OutOrStdout(), found)
			}
			printOverlays(cmd, found)
			return nil
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "only this collection (name or id)")
	return cmd
}

func printOverlays(cmd *cobra.Command, found []jsonOverlay) {
	out := cmd.OutOrStdout()
	if len(found) == 0 {
		fmt.Fprintln(out, "No flowi overlays. Save a request's edit as one from the app, or write overlays/<name>.flowi-overlay.yaml.")
		return
	}
	for _, o := range found {
		fmt.Fprintf(out, "%s: %s  (%d shared change(s), %d request(s))\n", o.Collection, o.Name, o.Shared, len(o.Requests))
		if o.Description != "" {
			fmt.Fprintf(out, "    %s\n", o.Description)
		}
	}
}

func newOverlayShowCommand(i do.Injector) *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Print a flowi overlay's file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			c, err := s.Workspace.ResolveCollection(collection)
			if err != nil {
				return err
			}
			o, err := s.Workspace.Overlay(c.ID, args[0])
			if err != nil {
				return err
			}
			data, err := os.ReadFile(o.File)
			if err != nil {
				return fmt.Errorf("reading %s: %w", o.File, err)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "the collection to look in (name or id)")
	return cmd
}

func newOverlayRmCommand(i do.Injector) *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Delete a flowi overlay (the responses kept for it stay)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			c, err := s.Workspace.ResolveCollection(collection)
			if err != nil {
				return err
			}
			if err := s.Workspace.DeleteOverlay(c.ID, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted flowi overlay %s.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "the collection to look in (name or id)")
	return cmd
}

func newOverlayExportCommand(i do.Injector) *cobra.Command {
	var collection, to string
	cmd := &cobra.Command{
		Use:   "export <name>",
		Short: "Write a flowi overlay as OpenAPI Overlays for the specs the collection keeps",
		Long: "Writes the parts of a flowi overlay that a spec can hold (an operation's docs, parameters and JSON\n" +
			"body example) as an OpenAPI Overlay, one document for each kept spec the overlay changes. What a spec\n" +
			"has no place for (checks, extractions, hooks, settings, variables, a new URL) stays in the flowi overlay\n" +
			"and is listed on standard error. With one document and no --to it is printed; --to <dir> writes\n" +
			"<dir>/<overlay>.<spec>.overlay.yaml.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session(cmd, i)
			if err != nil {
				return err
			}
			c, err := s.Workspace.ResolveCollection(collection)
			if err != nil {
				return err
			}
			res, err := s.Workspace.ExportOverlayAsSpec(c.ID, args[0])
			if err != nil {
				return err
			}
			reportNotCarried(cmd, res.NotCarried)
			return writeSpecOverlays(cmd, args[0], to, res.Documents)
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "the collection to look in (name or id)")
	cmd.Flags().StringVar(&to, "to", "", "a folder to write the OpenAPI Overlays to (needed when the overlay changes several specs)")
	return cmd
}

func reportNotCarried(cmd *cobra.Command, notes []flowi.ExportNote) {
	for _, n := range notes {
		where := n.Request
		if where == "" {
			where = "overlay"
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "not carried: %s: %s (%s)\n", where, n.What, n.Why)
	}
}

// writeSpecOverlays prints a single document or writes each to dir.
func writeSpecOverlays(cmd *cobra.Command, name, dir string, docs []flowi.SpecOverlayDocument) error {
	switch {
	case len(docs) == 0:
		fmt.Fprintln(cmd.ErrOrStderr(), "Nothing in this overlay can go into a spec, so no OpenAPI Overlay was written.")
		return nil
	case dir == "" && len(docs) == 1:
		_, err := fmt.Fprint(cmd.OutOrStdout(), docs[0].YAML)
		return err
	case dir == "":
		return fmt.Errorf("the overlay changes %d specs; give --to <folder> to write one OpenAPI Overlay for each", len(docs))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a folder for an export the user asked for, readable like their other files
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	for _, d := range docs {
		file := filepath.Join(dir, name+"."+d.Spec.Name+".overlay.yaml")
		if err := os.WriteFile(file, []byte(d.YAML), 0o644); err != nil { //nolint:gosec // an export the user asked for, readable like their other files
			return fmt.Errorf("writing %s: %w", file, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", file)
	}
	return nil
}
