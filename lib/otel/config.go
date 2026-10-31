// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"fmt"
	"strings"

	"golang.org/x/net/http/httpguts"

	"github.com/githugt/flowi/lib/settings"
)

// ConfigFrom reads the otel.* settings. on is false when no endpoint is
// set, which is the default: nothing is exported.
func ConfigFrom(r settings.Resolved, version string) (cfg Config, on bool, err error) {
	endpoint := stringSetting(r, "otel.endpoint")
	if endpoint == "" {
		return Config{}, false, nil
	}
	if _, err := tracesURL(endpoint); err != nil {
		return Config{}, false, err
	}
	headers, err := parseHeaders(listSetting(r, "otel.headers"))
	if err != nil {
		return Config{}, false, err
	}
	propagate, _ := settingValue(r, "otel.propagate").(bool)
	return Config{Endpoint: endpoint, Headers: headers, Propagate: propagate, Version: version}, true, nil
}

// parseHeaders reads "Name=value" entries; the value may itself contain "=".
func parseHeaders(entries []string) (map[string]string, error) {
	out := make(map[string]string, len(entries))
	for i, e := range entries {
		name, value, ok := strings.Cut(e, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) {
			// The entry is not echoed: its value is usually a token.
			return nil, fmt.Errorf("otel.headers[%d] is not a valid Name=value header", i)
		}
		out[name] = value
	}
	return out, nil
}

func settingValue(r settings.Resolved, name string) any {
	v, _ := r.Get(name)
	return v.Value
}

func stringSetting(r settings.Resolved, name string) string {
	s, _ := settingValue(r, name).(string)
	return s
}

func listSetting(r settings.Resolved, name string) []string {
	l, _ := settingValue(r, name).([]string)
	return l
}
