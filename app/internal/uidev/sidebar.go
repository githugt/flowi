// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev

import (
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// What the sidebar has to show beyond the shop: more collections with nested folders (to fold, star and search), every
// kind of request (so each verb pill and type shows), a Drafts space, and a few files with problems. Made up, in
// memory, like the rest of the fixture.

func httpRequest(coll, id, path, name, method, url string) model.Request {
	return model.Request{Meta: model.Meta{ID: model.ID(id), Name: name}, Path: path, File: "/" + coll + "/requests/" + path + ".req.yaml",
		Versions: []model.RequestVersion{{Label: "1", Method: method, URL: url}}}
}

func versionRequest(coll, id, path, name string, v model.RequestVersion) model.Request {
	v.Label = "1"
	return model.Request{Meta: model.Meta{ID: model.ID(id), Name: name}, Path: path, File: "/" + coll + "/requests/" + path + ".req.yaml", Versions: []model.RequestVersion{v}}
}

// sidebarCollections are the collections added to the fixture for the sidebar: an identity-checking API with folders,
// a messaging and data collection with the other request types, and the Drafts space.
func sidebarCollections() []model.Collection {
	hv := model.Collection{
		Root:     "/hyperverge",
		Manifest: model.Manifest{Meta: model.Meta{ID: "C-hv", Name: "Hyperverge", Description: "Identity checks: OCR, face match and liveness."}, File: "/hyperverge/flowi.yaml"},
		Folders: []model.Folder{
			{Dir: "requests/face-match-india", Status: "testing", Tags: []string{"face"}, File: "/hyperverge/requests/face-match-india/_folder.yaml"},
		},
		Requests: []model.Request{
			httpRequest("hyperverge", "H-ocr-aadhaar", "ocr/aadhaar", "Read Aadhaar", "POST", "{{env.base_url}}/ocr/aadhaar"),
			httpRequest("hyperverge", "H-ocr-pan", "ocr/pan", "Read PAN", "POST", "{{env.base_url}}/ocr/pan"),
			httpRequest("hyperverge", "H-face-match", "face-match-india/match", "Match a face", "POST", "{{env.base_url}}/face/match"),
			httpRequest("hyperverge", "H-face-deep", "face-match-india/selfie/compare", "Compare to a selfie", "POST", "{{env.base_url}}/face/selfie"),
			httpRequest("hyperverge", "H-live", "liveness-check/check", "Check liveness", "POST", "{{env.base_url}}/liveness"),
			httpRequest("hyperverge", "H-valid", "validation-api-india/pan", "Validate a PAN", "GET", "{{env.base_url}}/validate/pan"),
			httpRequest("hyperverge", "H-token", "token", "Get a token", "POST", "{{env.base_url}}/oauth/token"),
		},
		Flows: []model.Flow{{Meta: model.Meta{ID: "H-flow", Name: "Onboard", Description: "Reads the document, then matches the face."}, Path: "onboard", File: "/hyperverge/flows/onboard.arazzo.yaml",
			Nodes: []model.Node{{ID: "read", Type: "request"}, {ID: "match", Type: "request"}, {ID: "done", Type: "log"}}, Tags: []string{"smoke"}}},
	}
	hv.Requests[0].Tags = []string{"ocr"}
	hv.Requests[0].Versions[0].Docs = "# Read Aadhaar\n\nExtracts the name and number from a photo of the card.\n"
	hv.Requests[6].Status = "production"

	msg := model.Collection{
		Root:     "/messaging",
		Manifest: model.Manifest{Meta: model.Meta{ID: "C-msg", Name: "Test Collection"}, File: "/messaging/flowi.yaml"},
		Requests: []model.Request{
			versionRequest("messaging", "M-nats", "messaging/rabbitmq-message", "rabbitmq-message", model.RequestVersion{Async: &model.Async{Protocol: model.ProtocolNATS, Server: "nats://broker:4222", Channel: "orders.created"}}),
			versionRequest("messaging", "M-ws", "messaging/untitled", "untitled", model.RequestVersion{Async: &model.Async{Protocol: model.ProtocolWS, Server: "wss://echo.example/socket"}}),
			versionRequest("messaging", "M-mqtt", "messaging/sensors/temperature", "temperature", model.RequestVersion{Async: &model.Async{Protocol: model.ProtocolMQTT, Server: "mqtt://broker:1883", Channel: "sensors/temp"}}),
			versionRequest("messaging", "M-pg", "datasets/postgres-call", "postgres-call", model.RequestVersion{Dataset: &model.Dataset{Driver: "postgres", Server: "postgres://app:{{secret.db_password}}@db.example:5432/shop", Query: "SELECT id, total\nFROM orders\nWHERE total > $1", Params: []string{"100"}}}),
			versionRequest("messaging", "M-csv", "datasets/sample-csv", "sample-csv", model.RequestVersion{File: &model.DataFile{Format: "csv", Path: "data/people.csv", Query: "SELECT * FROM people"}}),
			versionRequest("messaging", "M-gql", "graphql/get-user", "get-user", model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/graphql", Body: model.Body{GraphQL: &model.GraphQL{
				Query: "query GetUser($id: ID!) {\n  user(id: $id) {\n    name\n    email\n  }\n}\n\nquery Broken {\n  broken\n}", Variables: "{\"id\": \"{{env.user_id}}\"}", Operation: "Broken"}}}),
			versionRequest("messaging", "M-gqlsub", "graphql/ticks", "ticks", model.RequestVersion{URL: "{{env.base_url}}/graphql", Body: model.Body{GraphQL: &model.GraphQL{
				Query: "subscription Ticks($from: Int) {\n  tick(from: $from) {\n    n\n  }\n}", Variables: "{\"from\": 1}", Operation: "Ticks", Receive: &model.AsyncReceive{Count: 3, Timeout: 10 * time.Second}}}}),
			versionRequest("messaging", "M-grpc", "grpc/ExampleService/GetExample", "GetExample", model.RequestVersion{GRPC: &model.GRPC{Server: "grpc://localhost:50051", Method: "example.ExampleService/GetExample"}}),
			versionRequest("messaging", "M-grpc2", "grpc/ExampleService/ListExamples", "ListExamples", model.RequestVersion{GRPC: &model.GRPC{Server: "grpc://localhost:50051", Method: "example.ExampleService/ListExamples"}}),
			versionRequest("messaging", "M-grpc3", "grpc/Health/Check", "Check", model.RequestVersion{GRPC: &model.GRPC{Server: "grpc://localhost:50051", Method: "grpc.health.v1.Health/Check"}}),
		},
	}
	msg.Requests[0].Status = "testing"

	drafts := model.Collection{
		Root:     "/drafts/scratch",
		Manifest: model.Manifest{Meta: model.Meta{ID: vmtest.ScratchID, Name: "Scratch"}, File: "/drafts/scratch/flowi.yaml"},
		Requests: []model.Request{
			httpRequest("drafts", "S-poke", "pokeapi-ditto", "pokeapi - ditto", "GET", "https://pokeapi.co/api/v2/pokemon/ditto"),
			httpRequest("drafts", "S-hook", "webhooksite-new", "webhooksite-new", "POST", "https://webhook.site/abc"),
			httpRequest("drafts", "S-replay", "replay", "replay", "POST", "https://example.test/replay"),
		},
		Flows: []model.Flow{{Meta: model.Meta{ID: "S-flow", Name: "untitled"}, Path: "untitled", File: "/drafts/scratch/flows/untitled.arazzo.yaml", Nodes: []model.Node{{ID: "a", Type: "log"}, {ID: "b", Type: "log"}, {ID: "c", Type: "log"}, {ID: "d", Type: "log"}}}},
	}
	return []model.Collection{hv, msg, drafts}
}

// sidebarProblems are validation problems on a few of the files above: an unknown key and a status the collection lacks.
func sidebarProblems() diag.List {
	return diag.List{
		diag.New(diag.CodeYAMLUnknownKey, diag.Location{File: "/hyperverge/requests/ocr/pan.req.yaml", Pos: diag.Position{Line: 7, Column: 3}}, map[string]string{"key": "metod"}),
		diag.New(diag.CodeStatusUnknown, diag.Location{File: "/demo/requests/orders/list.req.yaml"}, map[string]string{"status": "blocked", "where": "List orders", "known": "planning, in-development, review, testing, production, needs-design"}),
	}
}
