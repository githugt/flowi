// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	yamlcodec "github.com/githugt/flowi/lib/internal/codec/yaml"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// UserSettings resolves settings from the user file, FLOWI_* variables and
// flags only, without opening (or creating) a workspace. Applications use it
// for personal settings needed before a workspace exists, such as logging.
func UserSettings(p settings.Paths, environ []string, flags settings.LayerValues) (settings.Resolved, diag.List, error) {
	schema := settings.Schema()
	user, diags, err := readUserLayer(UserSettingsFile(p), yamlcodec.New(schema), schema)
	if err != nil {
		return settings.Resolved{}, nil, err
	}
	envLayer, envDiags := settings.FromEnviron(schema, environ)
	return settings.Resolve(schema, user, envLayer, flags), slices.Concat(diags, envDiags), nil
}

// readUserLayer reads settings.yaml. A missing, empty or comment-only file
// means "no overrides".
func readUserLayer(file string, codec *yamlcodec.Codec, schema []settings.Key) (settings.LayerValues, diag.List, error) {
	lv := settings.LayerValues{Layer: settings.LayerUser, Source: file, Values: model.Settings{}}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return lv, nil, nil
	}
	if err != nil {
		return lv, nil, fmt.Errorf("reading %s: %w", file, err)
	}
	raw, diags := codec.DecodeSettingsFile(file, data)
	diags = slices.DeleteFunc(diags, func(d diag.Diagnostic) bool { return d.Code == diag.CodeYAMLEmpty })
	lv, valueDiags := settings.FromBlock(schema, settings.LayerUser, file, raw)
	for _, d := range valueDiags {
		if pos, ok := codec.Locate(data, d.Path); ok {
			d.Pos = pos
		}
		diags = append(diags, d)
	}
	return lv, diags, nil
}

// UserSettingsFile is where the user's personal settings live.
func UserSettingsFile(p settings.Paths) string {
	return filepath.Join(p.ConfigDir, "settings.yaml")
}
