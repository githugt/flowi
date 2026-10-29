// SPDX-License-Identifier: Apache-2.0

package model

// Dependency is another collection this one references.
type Dependency struct {
	ID   ID
	Name string
	Repo string // optional clone URL shown when the dependency is not mounted
}

// Manifest is the content of a collection's flowi.yaml.
type Manifest struct {
	Format int
	Meta
	DependsOn []Dependency
	// Variables are the collection's own values, read as {{collection_var.name}}
	// by its requests and flows. The selected environment overrides one of
	// the same name, so the value that differs per API (a base URL) can still
	// differ per environment.
	Variables []KeyValue
	Settings  Settings
	Hooks     Hooks
	File      string
}

// Folder holds the settings and hooks of one _folder.yaml. Dir is relative
// to the collection root and slash-separated, e.g. "requests/auth".
type Folder struct {
	Dir      string
	Settings Settings
	Hooks    Hooks
	// Order lists the names of the folder's entries (a request or flow without its
	// file suffix, or a sub-folder's name) in the order they are shown. Entries it
	// does not name follow, by name; a name that matches nothing is ignored.
	Order []string
	// Tags apply to every flow and request under the folder.
	Tags []string
	File string
}

// Collection is everything loaded from one collection directory.
type Collection struct {
	Root         string
	Manifest     Manifest
	Folders      []Folder
	Environments []Environment
	Requests     []Request
	Flows        []Flow
	Blueprints   []Blueprint
	Overlays     []Overlay
}
