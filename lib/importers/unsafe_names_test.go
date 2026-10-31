// SPDX-License-Identifier: Apache-2.0

package importers_test

import (
	"path"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/importers/hoppscotch"
	"github.com/githugt/flowi-lib/importers/insomnia"
	"github.com/githugt/flowi-lib/importers/openrpc"
	"github.com/githugt/flowi-lib/importers/postman"
	"github.com/githugt/flowi-lib/importers/thunderclient"
)

const hostileName = `../../etc/pass wd/..\x:*?"<>|` + "\x00"

// A request or folder name comes from the file being imported, and becomes a
// path inside the collection. None may climb out of it or hold a character a
// file system refuses.
func TestImport_should_turn_a_hostile_request_name_into_a_safe_path_in_every_importer(t *testing.T) {
	importersUnderTest := map[string]func(string) (importers.Result, diag.List){
		"postman": func(name string) (importers.Result, diag.List) {
			return postman.Import([]byte(`{"info":{"name":"c","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[{"name":"`+name+`","item":[{"name":"`+name+`","request":{"method":"GET","url":"https://example.com"}}]}]}`), "c.json")
		},
		"insomnia": func(name string) (importers.Result, diag.List) {
			return insomnia.Import([]byte(`{"_type":"export","resources":[{"_id":"g","_type":"request_group","parentId":"w","name":"`+name+`"},{"_id":"r","_type":"request","parentId":"g","name":"`+name+`","method":"GET","url":"https://example.com"}]}`), "i.json")
		},
		"thunderclient": func(name string) (importers.Result, diag.List) {
			return thunderclient.Import([]byte(`{"clientName":"Thunder Client","folders":[{"_id":"g","name":"`+name+`","containerId":""}],"requests":[{"_id":"r","containerId":"g","name":"`+name+`","url":"https://example.com","method":"GET"}]}`), "t.json")
		},
		"openrpc": func(name string) (importers.Result, diag.List) {
			return openrpc.Import([]byte(`{"openrpc":"1.2.6","info":{},"methods":[{"name":"`+name+`","params":[]}]}`), "o.json")
		},
		"hoppscotch": func(name string) (importers.Result, diag.List) {
			return hoppscotch.Import([]byte(`[{"v":11,"name":"c","requests":[],"folders":[{"v":11,"name":"`+name+`","requests":[{"v":"13","name":"`+name+`","method":"GET","endpoint":"https://example.com","params":[],"headers":[]}],"folders":[]}]}]`), "h.json")
		},
	}
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\x00", `\u0000`).Replace(hostileName)

	for name, run := range importersUnderTest {
		t.Run(name, func(t *testing.T) {
			r, fatal := run(escaped)

			if fatal.HasErrors() || len(r.Requests) != 1 {
				t.Fatalf("fatal %v, %d requests", fatal, len(r.Requests))
			}
			p := r.Requests[0].Path
			if path.IsAbs(p) || strings.ContainsAny(p, "\\:*?\"<>|\x00") || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
				t.Errorf("path %q is not a safe path inside the collection", p)
			}
		})
	}
}
