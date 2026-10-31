// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
	"github.com/githugt/flowi/lib/store"
)

// Scope selects which project layers apply when resolving settings. The
// zero value resolves user and workspace settings only. Folder is
// collection-relative, e.g. "requests/auth".
type Scope struct {
	Collection model.ID
	Folder     string
}

// Settings resolves every setting for a scope.
func (w *Workspace) Settings(s Scope) settings.Resolved {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return settings.Resolve(w.schema, w.settingsLayers(s)...)
}

// settingsLayers is every layer for a scope except node and request ones.
// Callers hold mu.
func (w *Workspace) settingsLayers(s Scope) []settings.LayerValues {
	layers := []settings.LayerValues{
		w.user,
		{Layer: settings.LayerWorkspace, Source: w.model.File, Values: w.model.Settings},
	}
	if c, ok := w.collection(s.Collection); ok {
		layers = append(layers, settings.LayerValues{Layer: settings.LayerCollection, Source: c.Manifest.File, Values: c.Manifest.Settings})
		layers = append(layers, folderLayers(c, s.Folder)...)
	}
	envLayer, _ := settings.FromEnviron(w.schema, w.opts.Environ) // problems were reported by OpenWorkspace
	return append(layers, envLayer, w.opts.Flags)
}

func (w *Workspace) collection(id model.ID) (model.Collection, bool) {
	i := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == id })
	if i < 0 {
		return model.Collection{}, false
	}
	return w.collections[i], true
}

// folderLayers returns the _folder.yaml layers enclosing dir, innermost
// first: equal layers resolve in the order given, so the innermost wins.
func folderLayers(c model.Collection, dir string) []settings.LayerValues {
	var out []settings.LayerValues
	for d := dir; d != "." && d != "/" && d != ""; d = path.Dir(d) {
		for _, f := range c.Folders {
			if f.Dir == d {
				out = append(out, settings.LayerValues{Layer: settings.LayerFolder, Source: f.File, Values: f.Settings})
			}
		}
	}
	return out
}

// SetSetting stores a value at the user or workspace layer. raw is parsed
// as from an environment variable (lists are comma-separated).
func (w *Workspace) SetSetting(layer settings.Layer, key, raw string) error {
	k, err := w.settableKey(layer, key)
	if err != nil {
		return err
	}
	v, parseErr := settings.ParseValue(k, raw)
	if parseErr != nil {
		return settingError(diag.CodeSettingInvalid, key, map[string]string{"detail": parseErr.Error()})
	}
	return w.updateLayer(layer, func(s model.Settings) { s[key] = v })
}

// ResetSetting removes a value from the user or workspace layer.
func (w *Workspace) ResetSetting(layer settings.Layer, key string) error {
	if _, err := w.settableKey(layer, key); err != nil {
		return err
	}
	return w.updateLayer(layer, func(s model.Settings) { delete(s, key) })
}

func (w *Workspace) settableKey(layer settings.Layer, key string) (settings.Key, error) {
	k, ok := settings.Lookup(w.schema, key)
	if !ok {
		return k, settingError(diag.CodeSettingUnknown, key, nil)
	}
	allowed := layer == settings.LayerUser || layer == settings.LayerWorkspace
	if !allowed || !slices.Contains(k.FileLayers, layer) {
		return k, settingError(diag.CodeSettingWrongLayer, key, map[string]string{"layer": layer.String()})
	}
	return k, nil
}

// settingError builds an error-severity diagnostic: an unknown key is only
// a warning in a file, but a command that names one has failed.
func settingError(code diag.Code, key string, args map[string]string) error {
	all := map[string]string{"key": key}
	maps.Copy(all, args)
	d := diag.New(code, diag.Location{}, all)
	d.Severity = diag.SeverityError
	return diag.List{d}.Err()
}

func (w *Workspace) updateLayer(layer settings.Layer, change func(model.Settings)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if layer == settings.LayerWorkspace {
		next := w.model
		next.Settings = maps.Clone(next.Settings)
		if next.Settings == nil {
			next.Settings = model.Settings{}
		}
		change(next.Settings)
		return w.saveModel(next)
	}
	if len(w.userProblems) > 0 {
		return w.positioned(w.userProblems).Err()
	}
	values := maps.Clone(w.user.Values)
	change(values)
	data, err := w.codec.EncodeSettingsFile(values)
	if err != nil {
		return fmt.Errorf("encoding user settings: %w", err)
	}
	if strings.TrimSpace(string(data)) == "{}" {
		data = nil
	}
	if err := store.Apply([]store.FileChange{{File: w.userSettingsFile(), New: data}}); err != nil {
		return fmt.Errorf("saving user settings: %w", err)
	}
	w.user.Values = values
	return nil
}
