// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// DecodeAuth reads an auths/*.auth.yaml file: a named authentication, which
// is the auth block a request version would otherwise write itself.
func (c *Codec) DecodeAuth(file string, data []byte) (model.NamedAuth, diag.List) {
	a := model.NamedAuth{File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return a, d.diags
	}
	m, ok := d.mapping(root, "$", "authentication")
	if !ok {
		return a, d.diags
	}
	a.Meta = m.meta()
	if auth := m.auth(); auth != nil {
		a.Auth = *auth
	}
	m.closeStrict()
	return a, d.diags
}

// EncodeAuth writes a named authentication in canonical form.
func (c *Codec) EncodeAuth(a model.NamedAuth) ([]byte, error) {
	var o omap
	putMeta(&o, a.Meta)
	o.putAlways("auth", encodeAuth(a.Auth))
	return marshal(o.items)
}
