// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Languages a body or snippet is shown in.
const (
	LangPlain = "plain"
	LangJSON  = "json"
	LangXML   = "xml"   // XML and HTML
	LangShell = "shell" // the snippets: curl, wget, xh and PowerShell
)

// DetectLanguage says what text is by its content type, else by how it starts.
func DetectLanguage(contentType, text string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return LangJSON
	case strings.Contains(ct, "xml"), strings.Contains(ct, "html"):
		return LangXML
	}
	switch trimmed := strings.TrimSpace(text); {
	case strings.HasPrefix(trimmed, "{"), strings.HasPrefix(trimmed, "["):
		return LangJSON
	case strings.HasPrefix(trimmed, "<"):
		return LangXML
	}
	return LangPlain
}

// PrettyJSON indents valid JSON; ok is false, and text comes back as it was,
// for anything else.
func PrettyJSON(text string) (pretty string, ok bool) {
	if !json.Valid([]byte(text)) {
		return text, false
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(text), "", "  "); err != nil {
		return text, false
	}
	return out.String(), true
}
