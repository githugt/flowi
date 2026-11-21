// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// The hosts that may receive secrets besides those the selected environment names: the person's own setting
// secrets.allowed_hosts (see engine's secret destination check). These keep the list in the user layer, one host
// at a time, so allowing one does not replace the others.

const secretHostsKey = "secrets.allowed_hosts"

// AllowedSecretHosts lists the hosts the person has allowed to receive secrets, in the order they were added.
func (w *Workspace) AllowedSecretHosts() []string {
	v, _ := w.Settings(Scope{}).Get(secretHostsKey)
	switch list := v.Value.(type) {
	case []string:
		return slices.Clone(list)
	case []any:
		out := make([]string, 0, len(list))
		for _, e := range list {
			if h, ok := e.(string); ok {
				out = append(out, h)
			}
		}
		return out
	}
	return nil
}

// AllowSecretHost lets secrets go to host: "api.example.com" (any port), "api.example.com:8443" (that port) or
// "*.example.com" (any subdomain). An address (https://api.example.com/x) is cut down to its host. Allowing a host
// twice changes nothing; the host as stored is returned.
func (w *Workspace) AllowSecretHost(host string) (string, error) {
	h, err := cleanSecretHost(host)
	if err != nil {
		return "", err
	}
	list := w.AllowedSecretHosts()
	if slices.Contains(list, h) {
		return h, nil
	}
	return h, w.updateLayer(settings.LayerUser, func(s model.Settings) { s[secretHostsKey] = append(list, h) })
}

// DenySecretHost takes host off the allowed list; false when it was not on it.
func (w *Workspace) DenySecretHost(host string) (bool, error) {
	h, err := cleanSecretHost(host)
	if err != nil {
		return false, err
	}
	list := w.AllowedSecretHosts()
	i := slices.Index(list, h)
	if i < 0 {
		return false, nil
	}
	list = slices.Delete(list, i, i+1)
	return true, w.updateLayer(settings.LayerUser, func(s model.Settings) { s[secretHostsKey] = list })
}

// cleanSecretHost reads what the person typed as a host entry: lower case, with a port or a leading "*." when given.
func cleanSecretHost(raw string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(h, "://") {
		u, err := url.Parse(h)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("%q is not an address", raw)
		}
		h = u.Host
	}
	if i := strings.IndexAny(h, "/?#@ ,"); i >= 0 {
		return "", fmt.Errorf("%q is not a host: use api.example.com, api.example.com:8443 or *.example.com", raw)
	}
	if h == "" || h == "*" || strings.HasPrefix(h, "*") && !strings.HasPrefix(h, "*.") || strings.Count(h, "*") > 1 || strings.Contains(strings.TrimPrefix(h, "*."), "*") {
		return "", errors.New("name a host, such as api.example.com, api.example.com:8443 or *.example.com (a bare * would allow every host)")
	}
	return h, nil
}
