// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/githugt/flowi/lib/settings"
)

// GlobalOptions are the persistent flags shared by every command.
type GlobalOptions struct {
	ConfigDir string
	LogLevel  string
	LogFile   string
	Output    string
	Color     string // auto, always or never
}

// Env is what the process knows about its surroundings.
type Env struct {
	Environ []string // from settings.ProcessEnviron
	Home    string
	Cwd     string
}

func addGlobalFlags(root *cobra.Command, g *GlobalOptions) {
	f := root.PersistentFlags()
	f.StringVar(&g.ConfigDir, "config-dir", "", "directory for Flowi's own files (default: FLOWI_CONFIG_DIR or the OS config dir)")
	f.StringVar(&g.LogLevel, "log-level", "", "log level: debug, info, warn or error")
	f.StringVar(&g.LogFile, "log-file", "", "append logs to this file instead of standard error")
	f.StringVar(&g.Output, "output", OutputText, "output format: text, json or github (workflow annotations; also automatic when GITHUB_ACTIONS is set); FLOWI_OUTPUT sets the default")
	f.StringVar(&g.Color, "color", ColorAuto, "colour in live progress: auto (a terminal, unless NO_COLOR is set), always or never")
}

// Colour modes of --color.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// applyEnv takes the output format from FLOWI_OUTPUT when --output was not given, so a CI job can set it once.
//
// Before: the format could only be chosen on each command line.
func (g *GlobalOptions) applyEnv(environ []string, outputGiven bool) {
	if outputGiven {
		return
	}
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "FLOWI_OUTPUT="); ok && v != "" {
			g.Output = v
		}
	}
}

func (g *GlobalOptions) validate() error {
	if g.Color != ColorAuto && g.Color != ColorAlways && g.Color != ColorNever {
		return &UsageError{Err: fmt.Errorf("unknown colour mode %q (want auto, always or never)", g.Color)}
	}
	if g.Output != OutputText && g.Output != OutputJSON && g.Output != OutputGitHub {
		return &UsageError{Err: fmt.Errorf("unknown output format %q (want text, json or github)", g.Output)}
	}
	return nil
}

// FlagLayer turns the logging flags into the highest settings layer.
func (g *GlobalOptions) FlagLayer() settings.LayerValues {
	values := map[string]any{}
	if g.LogLevel != "" {
		values["logging.level"] = g.LogLevel
	}
	if g.LogFile != "" {
		values["logging.file"] = g.LogFile
	}
	return settings.LayerValues{Layer: settings.LayerFlag, Source: "command line", Values: values}
}

// Paths returns Flowi's directories, honouring --config-dir.
func (g *GlobalOptions) Paths(environ []string) (settings.Paths, error) {
	if g.ConfigDir != "" {
		return settings.Paths{ConfigDir: g.ConfigDir}, nil
	}
	return settings.DiscoverPaths(environ)
}
