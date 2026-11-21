// SPDX-License-Identifier: Apache-2.0

package flowi

import "testing"

func TestDetectImportKind_Given_a_document_of_each_kind_Then_the_kind_and_a_description_come_back(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		head  string
		kind  ImportKind
		label string
	}{
		{"openapi json", "pets.json", `{"openapi": "3.1.0", "info": {}}`, ImportOpenAPI, "OpenAPI 3.1.0"},
		{"openapi yaml", "api.yaml", "openapi: 3.0.3\ninfo:\n  title: x\n", ImportOpenAPI, "OpenAPI 3.0.3"},
		{"swagger quoted", "s.yaml", "swagger: '2.0'\ninfo: {}\n", ImportOpenAPI, "Swagger 2.0"},
		{"asyncapi", "events.yaml", "asyncapi: 2.6.0\ninfo: {}\n", ImportAsyncAPI, "AsyncAPI 2.6.0"},
		{"arazzo", "flows.arazzo.yaml", "arazzo: 1.0.1\ninfo: {}\n", ImportArazzo, "Arazzo 1.0.1"},
		{"openrpc", "rpc.json", `{"openrpc": "1.2.6", "methods": []}`, ImportOpenRPC, "OpenRPC 1.2.6"},
		{"har", "capture.har", `{"log": {"version": "1.2", "entries": []}}`, ImportHAR, "a HAR file"},
		{"postman collection", "c.json", `{"info": {"_postman_id": "abc", "name": "x"}, "item": []}`, ImportPostman, "a Postman export"},
		{"postman by schema", "c.json", `{"info": {"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"}}`, ImportPostman, "a Postman export"},
		{"insomnia 4", "i.json", `{"_type": "export", "__export_format": 4, "resources": []}`, ImportInsomnia, "an Insomnia export"},
		{"insomnia 5", "i.yaml", "type: collection.insomnia.rest/5.0\nname: x\n", ImportInsomnia, "an Insomnia export"},
		{"thunder client", "tc.json", `{"clientName": "Thunder Client", "collectionName": "x", "requests": []}`, ImportThunderClient, "a Thunder Client export"},
		{"hoppscotch", "h.json", `[{"v": 1, "name": "x", "folders": [], "requests": []}]`, ImportHoppscotch, "a Hoppscotch export"},
		{"proto by extension", "pets.proto", "package pets;\n", ImportProto, "a gRPC .proto file"},
		{"proto by content", "", "syntax = \"proto3\";\nmessage Pet { string id = 1; }\n", ImportProto, "a gRPC .proto file"},
		{"http by extension", "calls.http", "### one\nGET https://x.test/pets\n", ImportHTTP, "a .http file"},
		{"http by content", "", "# comment\nPOST {{base}}/pets HTTP/1.1\nContent-Type: application/json\n", ImportHTTP, "a .http file"},
		{"curl", "", "curl -X POST https://x.test/pets -d '{}'", ImportCurl, "a curl command"},
		{"curl with a prompt", "", "$ curl https://x.test", ImportCurl, "a curl command"},
		{"mermaid", "flow.mmd", "flowchart TD\n  a --> b\n", ImportMermaid, "a Mermaid flowchart"},
		{"mermaid by content", "", "graph LR\n  a --> b\n", ImportMermaid, "a Mermaid flowchart"},
		{"with a byte order mark", "x.json", "\ufeff{\"openapi\": \"3.0.0\"}", ImportOpenAPI, "OpenAPI 3.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, ok := DetectImportKind(tt.file, []byte(tt.head))

			// Assert
			if !ok || got.Kind != tt.kind || got.Label != tt.label {
				t.Fatalf("got %+v ok %v, want %s %q", got, ok, tt.kind, tt.label)
			}
		})
	}
}

func TestDetectImportKind_Given_things_that_are_not_documents_Then_nothing_is_recognised(t *testing.T) {
	for name, head := range map[string]string{
		"empty":               "",
		"blank":               "  \n ",
		"an address":          "https://x.test/pets",
		"a json that is none": `{"hello": "world"}`,
		"prose":               "Remember to buy milk.",
		"a list of numbers":   `[1, 2, 3]`,
	} {
		t.Run(name, func(t *testing.T) {
			if got, ok := DetectImportKind("", []byte(head)); ok {
				t.Fatalf("recognised %+v", got)
			}
		})
	}
}

func TestDetectImportDir_Given_a_folder_with_bruno_json_Then_it_is_a_bruno_collection(t *testing.T) {
	got, ok := DetectImportDir([]string{"requests", "bruno.json"})
	none, noneOK := DetectImportDir([]string{"notes.txt"})

	if !ok || got.Kind != ImportBruno || noneOK || none.Kind != "" {
		t.Fatalf("got %+v %v none %+v %v", got, ok, none, noneOK)
	}
}
