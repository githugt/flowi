// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// Dir is the directory of the named workspace.
func Dir(p settings.Paths, name string) string {
	return filepath.Join(p.ConfigDir, "workspaces", name)
}

// EnvironmentsDir holds the workspace's environments. They belong to the
// workspace, not to a collection: every collection sees the same set, and
// the files are personal (never committed with a collection).
func EnvironmentsDir(p settings.Paths, name string) string {
	return filepath.Join(Dir(p, name), "environments")
}

// DataDir holds run history, saved responses and cookies for one
// collection in the named workspace.
func DataDir(p settings.Paths, name string, collection model.ID) string {
	return filepath.Join(Dir(p, name), "data", string(collection))
}

// ExpandPath replaces a leading "~" or "~/" with home. "~user" forms are
// left alone: Flow-I does not look up other users' home directories.
func ExpandPath(path, home string) string {
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, filepath.FromSlash(rest))
	}
	return path
}

// DirExists reports whether path is an existing directory.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// DraftsDir holds the named workspace's drafts: collections the user has
// not saved to a folder of their own yet.
func DraftsDir(p settings.Paths, name string) string {
	return filepath.Join(UserDir(p, name, DefaultUser), "drafts")
}

// TrashDir holds what was moved out of the drafts and not deleted yet.
func TrashDir(p settings.Paths, name string) string {
	return filepath.Join(UserDir(p, name, DefaultUser), "trash")
}

// DefaultUser is the user everything a person makes in the app belongs to.
// There is only one for now; the directory layout is ready for more.
const DefaultUser = "default"

// UserDir holds what belongs to one user of the named workspace.
func UserDir(p settings.Paths, name, user string) string {
	return filepath.Join(Dir(p, name), "users", user)
}

// LegacyDraftsDir is where drafts lived before they belonged to a user.
// Drafts already there stay there and are still drafts.
func LegacyDraftsDir(p settings.Paths, name string) string {
	return filepath.Join(Dir(p, name), "drafts")
}

// KeptDir holds the unsaved edits of flows that were kept when the app quit,
// one file per flow, until the flow is opened again.
func KeptDir(p settings.Paths, name string) string {
	return filepath.Join(UserDir(p, name, DefaultUser), "kept")
}
