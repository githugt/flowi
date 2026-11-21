// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"runtime"
	"strings"
)

// splitCommand splits an editor command such as `code --wait` or `"C:\Program Files\Vim\vim.exe" -u NONE` into
// the program and its arguments. Single and double quotes group words; a backslash escapes the next character
// except on Windows, where it is a path separator. It does not expand variables or run anything through a shell.
func splitCommand(s string) ([]string, error) {
	return splitCommandOS(s, runtime.GOOS == "windows")
}

func splitCommandOS(s string, windows bool) ([]string, error) {
	var (
		parts   []string
		cur     strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && !windows && quote != '\'':
			escaped, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inWord {
				parts = append(parts, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, errors.New("an unclosed quote in the editor command")
	}
	if escaped {
		cur.WriteRune('\\')
	}
	if inWord {
		parts = append(parts, cur.String())
	}
	return parts, nil
}
