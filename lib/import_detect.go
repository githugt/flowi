// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"path/filepath"
	"regexp"
	"strings"
)

// DetectHeadBytes is how much of a document DetectImportKind needs: the markers it looks for sit near the start.
const DetectHeadBytes = 64 << 10

// Detected is what DetectImportKind found: the kind and a short description for people ("OpenAPI 3.1").
type Detected struct {
	Kind  ImportKind
	Label string
}

var (
	reOpenAPI   = regexp.MustCompile(`(?m)(?:^|[{,])\s*"?(openapi|swagger)"?\s*:\s*["']?(\d[\w.\-]*)`)
	reAsyncAPI  = regexp.MustCompile(`(?m)(?:^|[{,])\s*"?asyncapi"?\s*:\s*["']?(\d[\w.\-]*)`)
	reArazzo    = regexp.MustCompile(`(?m)(?:^|[{,])\s*"?arazzo"?\s*:\s*["']?(\d[\w.\-]*)`)
	reOpenRPC   = regexp.MustCompile(`(?m)(?:^|[{,])\s*"?openrpc"?\s*:\s*["']?(\d[\w.\-]*)`)
	reHARLog    = regexp.MustCompile(`"log"\s*:\s*\{`)
	reHAREntry  = regexp.MustCompile(`"entries"\s*:\s*\[`)
	rePostman   = regexp.MustCompile(`"_postman_id"|getpostman\.com|"_postman_variable_scope"`)
	reInsomnia4 = regexp.MustCompile(`"_type"\s*:\s*"export"`)
	reInsomnia5 = regexp.MustCompile(`(?m)(?:^|[{,])\s*"?type"?\s*:\s*["']?collection\.insomnia\.rest/`)
	reThunder   = regexp.MustCompile(`"(clientName|collectionName)"\s*:|"client"\s*:\s*"Thunder Client"`)
	reHopp      = regexp.MustCompile(`"v"\s*:\s*\d+`)
	reHoppShape = regexp.MustCompile(`"(folders|requests|variables)"\s*:\s*\[`)
	reProto     = regexp.MustCompile(`(?m)^\s*(syntax\s*=\s*["']proto[23]["']|(message|service|enum)\s+\w+\s*\{|import\s+["'][^"']+\.proto["'])`)
	reMethod    = regexp.MustCompile(`(?m)^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|QUERY)\s+(https?://|\{\{)\S*(\s+HTTP/[\d.]+)?\s*$`)
	reCurl      = regexp.MustCompile(`(?i)^(\$\s*)?curl\s`)
	reMermaid   = regexp.MustCompile(`(?im)^\s*(flowchart|graph)\s+(TB|TD|BT|RL|LR)\b`)
)

// DetectImportKind says what kind of document head (its first DetectHeadBytes bytes) is, using name (a file name, or
// "" for pasted text) as a hint: a .proto has no marker that is safe to trust on its own, a .http file is one by its
// extension. The markers are the ones each importer itself requires, so a detected kind is one the importer reads.
// ok is false when nothing is recognised: the person chooses the kind.
//
// Before: the window's own rules recognised six of the fourteen kinds and only for pasted text, in JavaScript the
// command line could not share.
func DetectImportKind(name string, head []byte) (Detected, bool) {
	text := strings.TrimPrefix(string(head), "\ufeff")
	t := strings.TrimSpace(text)
	if t == "" {
		return Detected{}, false
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case reCurl.MatchString(t):
		return Detected{ImportCurl, "a curl command"}, true
	case ext == ".proto" || (ext == "" && reProto.MatchString(text) && !strings.HasPrefix(t, "{")):
		return Detected{ImportProto, "a gRPC .proto file"}, true
	case ext == ".http" || ext == ".rest" || firstLineIsRequest(t):
		return Detected{ImportHTTP, "a .http file"}, true
	case ext == ".mmd" || ext == ".mermaid" || reMermaid.MatchString(firstLines(t, 3)):
		return Detected{ImportMermaid, "a Mermaid flowchart"}, true
	}
	if m := reArazzo.FindStringSubmatch(text); m != nil {
		return Detected{ImportArazzo, "Arazzo " + m[1]}, true
	}
	if m := reOpenRPC.FindStringSubmatch(text); m != nil {
		return Detected{ImportOpenRPC, "OpenRPC " + m[1]}, true
	}
	if m := reAsyncAPI.FindStringSubmatch(text); m != nil {
		return Detected{ImportAsyncAPI, "AsyncAPI " + m[1]}, true
	}
	if m := reOpenAPI.FindStringSubmatch(text); m != nil {
		label := "OpenAPI " + m[2]
		if m[1] == "swagger" {
			label = "Swagger " + m[2]
		}
		return Detected{ImportOpenAPI, label}, true
	}
	switch {
	case reHARLog.MatchString(text) && reHAREntry.MatchString(text):
		return Detected{ImportHAR, "a HAR file"}, true
	case rePostman.MatchString(text):
		return Detected{ImportPostman, "a Postman export"}, true
	case reInsomnia4.MatchString(text) || reInsomnia5.MatchString(text):
		return Detected{ImportInsomnia, "an Insomnia export"}, true
	case reThunder.MatchString(text) && strings.HasPrefix(t, "{"):
		return Detected{ImportThunderClient, "a Thunder Client export"}, true
	case (strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")) && reHopp.MatchString(text) && reHoppShape.MatchString(text):
		return Detected{ImportHoppscotch, "a Hoppscotch export"}, true
	}
	return Detected{}, false
}

// firstLineIsRequest is whether the first line that is not blank or a comment is a request line of a .http file,
// with more text after it (a bare address pastes as an address).
func firstLineIsRequest(t string) bool {
	if !strings.Contains(t, "\n") {
		return false
	}
	for _, line := range strings.Split(t, "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "//") {
			return reMethod.MatchString(l)
		}
	}
	return false
}

// firstLines is the first n lines of t.
func firstLines(t string, n int) string {
	lines := strings.SplitN(t, "\n", n+1)
	return strings.Join(lines[:min(len(lines), n)], "\n")
}

// DetectImportDir says whether dir holds a collection folder Flowi imports (a Bruno collection has bruno.json), by the
// names of the files in it.
func DetectImportDir(entries []string) (Detected, bool) {
	for _, e := range entries {
		if e == "bruno.json" || e == "opencollection.yml" {
			return Detected{ImportBruno, "a Bruno collection folder"}, true
		}
	}
	return Detected{}, false
}
