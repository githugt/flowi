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
	// MinVersion is the oldest Flowi release that can read this collection correctly, as "1.4.0"; "" for any.
	// A release older than it refuses the collection with a message that says which release to install.
	MinVersion string
	DependsOn  []Dependency
	// Variables are the collection's own values, read as {{collection_var.name}}
	// by its requests and flows. The selected environment overrides one of
	// the same name, so the value that differs per API (a base URL) can still
	// differ per environment.
	Variables []KeyValue
	// Statuses are the development statuses the collection adds to the defaults (DefaultStatuses), or whose colour and
	// icon it changes by using a default's name. Manifest.StatusList is the whole list.
	Statuses []Status
	Settings Settings
	Hooks    Hooks
	File     string
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
	// Status is the default development status of everything under the folder that does not set its own.
	Status string
	File   string
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
	Auths        []NamedAuth
	Overlays     []Overlay
	// Broken lists the request and flow files that could not be read at all (a syntax error, git's conflict markers),
	// so they are not among Requests and Flows. Before: such a file simply vanished from the collection, which
	// looked like a deleted one.
	Broken []BrokenFile
}

// BrokenFile is a request or flow file that could not be read.
type BrokenFile struct {
	File    string // the file's path on disk
	Path    string // its path in the collection, as a request or flow of that path would have
	Kind    string // "request" or "flow"
	Code    string // the diagnostic code that said why, e.g. FLW-FILE-003 for conflict markers
	Message string // that diagnostic in words, with the line when it has one
}
