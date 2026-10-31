// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "strings"

// maxSlugLen bounds a path made from a URL.
const maxSlugLen = 48

// DefaultSavePath suggests a path for saving a draft request: its own name, or
// for an untitled one the last parts of its URL joined with dashes (one name,
// not a folder per URL segment).
func DefaultSavePath(name, url string) string {
	if !strings.HasPrefix(name, "untitled") || url == "" {
		return name
	}
	var b strings.Builder
	for _, r := range trimURLScheme(url) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == '?' || r == '#':
			return clipSlug(b.String())
		default:
			b.WriteByte('-')
		}
	}
	return clipSlug(b.String())
}

func trimURLScheme(u string) string {
	for _, p := range []string{"https://", "http://"} {
		if rest, ok := strings.CutPrefix(u, p); ok {
			return rest
		}
	}
	return u
}

func clipSlug(s string) string {
	s = strings.Trim(s, "-")
	if len(s) > maxSlugLen {
		s = strings.Trim(s[len(s)-maxSlugLen:], "-")
	}
	if s == "" {
		return "request"
	}
	return s
}
