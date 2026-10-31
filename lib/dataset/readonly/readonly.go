// SPDX-License-Identifier: Apache-2.0

// Package readonly decides, from the text of a statement alone, whether a query only reads. It is the first of two
// guards: it gives a clear message before anything is sent, and the second is the read-only transaction the store itself
// enforces. It knows nothing of any driver, so the validator can use it without pulling the drivers in.
package readonly

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The reasons Check refuses a statement.
var (
	ErrEmptyStatement = errors.New("the query is empty")
	ErrManyStatements = errors.New("the query has more than one statement")
	ErrNotReadOnly    = errors.New("the query is not read-only")
)

// readingStatements are the statements that start a query that only reads. It is a list of what is allowed, not of
// what is refused: a statement the list does not know is refused.
func readingStatements() map[string]bool {
	return map[string]bool{"SELECT": true, "WITH": true, "VALUES": true, "TABLE": true, "SHOW": true, "EXPLAIN": true, "DESCRIBE": true, "DESC": true}
}

// writingInto is SELECT … INTO: a new table in some stores, a file on the server in MySQL.
func writingInto() *regexp.Regexp { return regexp.MustCompile(`(?i)\binto\b`) }

// Check refuses a statement that is empty, has more than one statement, or does not start like a read. A WITH
// that deletes passes it and is stopped by the read-only transaction every source opens.
func Check(statement string) error {
	code, semicolons, err := codeOf(statement)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrEmptyStatement
	}
	if semicolons > 0 {
		return ErrManyStatements
	}
	word, _, _ := strings.Cut(strings.ToUpper(code), " ")
	if !readingStatements()[word] {
		return fmt.Errorf("%w: it starts with %s, and only SELECT, WITH, VALUES, TABLE, SHOW, EXPLAIN and DESCRIBE are allowed", ErrNotReadOnly, word)
	}
	if writingInto().MatchString(code) {
		return fmt.Errorf("%w: INTO writes the result somewhere", ErrNotReadOnly)
	}
	return nil
}

// codeOf is the statement as the store reads it outside strings and comments: comments become a space, and what is
// inside quotes becomes blank so words in it are not taken for words of the statement. The semicolons that still
// separate statements after the last one are counted: a trailing ; is not one.
func codeOf(statement string) (code string, separators int, err error) {
	var out strings.Builder
	runes := []rune(statement)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '-' && at(runes, i+1) == '-':
			i = skipLine(runes, i)
			out.WriteByte(' ')
		case r == '/' && at(runes, i+1) == '*':
			end := skipBlock(runes, i)
			if end < 0 {
				return "", 0, fmt.Errorf("%w: a comment is never closed", ErrNotReadOnly)
			}
			i = end
			out.WriteByte(' ')
		case r == '\'' || r == '"' || r == '`':
			i = skipQuoted(runes, i)
			out.WriteString(" ")
		default:
			out.WriteRune(r)
		}
	}
	trimmed := strings.TrimRight(strings.TrimSpace(out.String()), "; \t\r\n")
	return trimmed, strings.Count(trimmed, ";"), nil
}

func at(runes []rune, i int) rune {
	if i < len(runes) {
		return runes[i]
	}
	return 0
}

// skipLine is the index of the end of the line that starts at from.
func skipLine(runes []rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == '\n' {
			return i
		}
	}
	return len(runes)
}

// skipBlock is the index of the / that closes the /* at from, or -1 when it is never closed.
func skipBlock(runes []rune, from int) int {
	for i := from + 2; i+1 < len(runes); i++ {
		if runes[i] == '*' && runes[i+1] == '/' {
			return i + 1
		}
	}
	return -1
}

// skipQuoted is the index of the quote that closes the one at from; a doubled quote inside stays in the string. An
// unclosed quote runs to the end.
func skipQuoted(runes []rune, from int) int {
	quote := runes[from]
	for i := from + 1; i < len(runes); i++ {
		if runes[i] != quote {
			continue
		}
		if at(runes, i+1) == quote {
			i++
			continue
		}
		return i
	}
	return len(runes)
}
