// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
)

// EmptyPolicy says what to do with a null or empty-string value in a
// header or query parameter (design §6.2).
type EmptyPolicy int

// Empty policies.
const (
	EmptyKeep EmptyPolicy = iota + 1
	EmptyOmit
	EmptyError
)

// ErrEmpty is returned by ApplyEmpty under EmptyError.
var ErrEmpty = errors.New("value is empty")

// ParseEmptyPolicy reads the empty setting ("keep", "omit", "error").
func ParseEmptyPolicy(s string) (EmptyPolicy, error) {
	policies := map[string]EmptyPolicy{"keep": EmptyKeep, "omit": EmptyOmit, "error": EmptyError}
	p, ok := policies[s]
	if !ok {
		return 0, fmt.Errorf("unknown empty policy %q", s)
	}
	return p, nil
}

// ApplyEmpty turns a header or query value into text. keep is false when
// the field should be left out.
func ApplyEmpty(v any, p EmptyPolicy) (value string, keep bool, err error) {
	if v != nil && v != "" {
		return stringify(v), true, nil
	}
	switch p {
	case EmptyKeep:
		return "", true, nil
	case EmptyError:
		return "", false, ErrEmpty
	}
	return "", false, nil
}
