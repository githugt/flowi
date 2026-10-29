// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// DecodeFolder reads a _folder.yaml. dir is the folder's path relative to
// the collection root, e.g. "requests/auth".
func (c *Codec) DecodeFolder(file, dir string, data []byte) (model.Folder, diag.List) {
	f := model.Folder{Dir: dir, File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return f, d.diags
	}
	m, ok := d.mapping(root, "$", "_folder.yaml")
	if !ok {
		return f, d.diags
	}
	f.Settings = c.settingsChild(d, m, settings.LayerFolder)
	f.Hooks = m.hooks()
	f.Order = m.stringList("order")
	f.Tags = m.stringList("tags")
	m.closeStrict()
	return f, d.diags
}

// EncodeFolder writes a _folder.yaml in canonical form.
func (c *Codec) EncodeFolder(f model.Folder) ([]byte, error) {
	var o omap
	o.put("settings", c.encodeSettings(f.Settings))
	o.put("hooks", encodeHooks(f.Hooks))
	o.put("order", f.Order)
	o.put("tags", f.Tags)
	return marshal(o.items)
}
