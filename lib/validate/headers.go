// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"golang.org/x/net/http/httpguts"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
)

// checkHeaderNames reports request headers whose name can never be sent: an
// empty name, or one with characters a header name cannot hold. A blank
// header row is almost always a mistake, so it is an error in the file, not
// something to skip silently at send time.
func checkHeaderNames(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				headers := yamlpath.Key(yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), "headers")
				for _, h := range v.Headers {
					detail := invalidHeaderNameDetail(h.Key)
					if detail == "" || strings.TrimSpace(h.Key) != h.Key && h.Key != "" {
						// Stray whitespace around a name is a lint warning the GUI can repair.
						continue
					}
					out = append(out, diag.New(diag.CodeRequestHeader, diag.Location{File: r.File, Path: headers}, map[string]string{"name": h.Key, "detail": detail}))
				}
			}
		}
	}
	return out
}

// invalidHeaderNameDetail returns why name is not a valid header name, or
// "" when it is. Names are used as written, so a template in a name is invalid.
func invalidHeaderNameDetail(name string) string {
	switch {
	case name == "":
		return "the name is empty"
	case !httpguts.ValidHeaderFieldName(name):
		return "the name is not a valid header name"
	}
	return ""
}
