// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Paths are the directories Flow-I keeps its own files in.
type Paths struct {
	ConfigDir string
}

// DiscoverPaths returns FLOWI_CONFIG_DIR if set, otherwise
// os.UserConfigDir()/flowi.
func DiscoverPaths(environ []string) (Paths, error) {
	if dir := lookupEnv(environ, "FLOWI_CONFIG_DIR"); dir != "" {
		return Paths{ConfigDir: dir}, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("finding user config directory (set FLOWI_CONFIG_DIR to override): %w", err)
	}
	return Paths{ConfigDir: filepath.Join(base, "flowi")}, nil
}

// Editor returns the user's editor command: $VISUAL, then $EDITOR, then
// a platform default.
func Editor(environ []string) string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if v := lookupEnv(environ, name); v != "" {
			return v
		}
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}
