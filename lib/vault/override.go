// SPDX-License-Identifier: Apache-2.0

package vault

import "strings"

const (
	envVaultKey     = "FLOWI_VAULT_KEY"
	envSecretPrefix = "FLOWI_SECRET_"
)

// OverrideName is the environment variable that overrides a secret:
// FLOWI_SECRET_<ENV>_<NAME>, upper-cased, with every character that is
// not a letter or digit replaced by "_".
func OverrideName(envName, name string) string {
	return envSecretPrefix + envSafe(envName) + "_" + envSafe(name)
}

func envSafe(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			return r
		}
		return '_'
	}, s)
}

// lookup finds key in environ ("K=V" entries); the last entry wins, as in
// a process environment.
func lookup(environ []string, key string) string {
	value := ""
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			value = v
		}
	}
	return value
}

// Overridden returns the FLOWI_SECRET_<ENV>_<NAME> value for a secret.
// An empty value counts as unset, as for every FLOWI_ variable.
func Overridden(environ []string, envName, name string) (string, bool) {
	prefix := OverrideName(envName, name) + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(environ[i], prefix); ok {
			return v, v != ""
		}
	}
	return "", false
}
