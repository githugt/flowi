// SPDX-License-Identifier: Apache-2.0

package har

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

const recordedSession = `{"log": {"entries": [
 {"_resourceType": "fetch", "request": {"method": "POST", "url": "https://api.example.com/v1/pets", "headers": [], "queryString": [],
   "postData": {"mimeType": "application/json", "text": "{\"name\":\"rex\"}"}},
  "response": {"status": 201, "headers": [], "content": {"mimeType": "application/json", "text": "{\"id\":7,\"owner\":{\"handle\":\"abcd1234\"}}"}}},
 {"_resourceType": "fetch", "request": {"method": "GET", "url": "https://api.example.com/v1/pets/7", "headers": [], "queryString": []},
  "response": {"status": 200, "headers": [], "content": {"mimeType": "application/json", "text": "{\"id\":7,\"name\":\"rex\"}"}}},
 {"_resourceType": "fetch", "request": {"method": "POST", "url": "https://api.example.com/v1/pets/7/shots?owner=abcd1234&page=2", "headers": [],
   "queryString": [{"name": "owner", "value": "abcd1234"}, {"name": "page", "value": "2"}],
   "postData": {"mimeType": "application/json", "text": "{\"pet\":7,\"note\":\"first shot\"}"}},
  "response": {"status": 201, "headers": [], "content": {"mimeType": "application/json", "text": "{\"id\":90}"}}}
]}}`

func nodeNamed(t *testing.T, nodes []model.Node, id string) model.Node {
	t.Helper()
	for _, n := range nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("no node %q in %+v", id, nodes)
	return model.Node{}
}

func withValue(n model.Node, key string) string {
	for _, kv := range n.Request.With {
		if kv.Key == key {
			return kv.Value
		}
	}
	return "<none>"
}

func requestAt(t *testing.T, requests []model.Request, node model.Node) model.RequestVersion {
	t.Helper()
	for _, r := range requests {
		if r.Path == node.Request.Request.Path {
			return r.Versions[0]
		}
	}
	t.Fatalf("no request for %q", node.ID)
	return model.RequestVersion{}
}

func TestImport_Given_a_value_from_an_earlier_response_in_a_path_Then_the_step_reads_it_from_that_step(t *testing.T) {
	// Act
	r, _ := Import([]byte(recordedSession), "pets.har")

	// Assert
	nodes := r.Flows[0].Nodes
	if got := withValue(nodeNamed(t, nodes, "get-pets"), "id"); got != "{{nodes.post-pets.body.id}}" {
		t.Errorf("get-pets id = %q", got)
	}
	if got := withValue(nodeNamed(t, nodes, "post-shots"), "id"); got != "{{nodes.post-pets.body.id}}" {
		t.Errorf("post-shots id = %q", got)
	}
}

func TestImport_Given_a_query_value_and_a_body_field_from_an_earlier_response_Then_they_become_with_parameters_that_read_it(t *testing.T) {
	// Act
	r, _ := Import([]byte(recordedSession), "pets.har")

	// Assert
	shots := nodeNamed(t, r.Flows[0].Nodes, "post-shots")
	if got := withValue(shots, "owner"); got != "{{nodes.post-pets.body.owner.handle}}" {
		t.Errorf("owner = %q", got)
	}
	if got := withValue(shots, "pet"); got != "{{nodes.post-pets.body.id}}" {
		t.Errorf("pet = %q", got)
	}
	v := requestAt(t, r.Requests, shots)
	var owner, page string
	for _, q := range v.Query {
		switch q.Key {
		case "owner":
			owner = q.Value
		case "page":
			page = q.Value
		}
	}
	if owner != "{{with.owner}}" || page != "2" {
		t.Errorf("query owner %q page %q: only the value that came from a response is parameterised", owner, page)
	}
	body := v.Body.JSON
	if !strings.Contains(body, `"pet":{{with.pet}}`) || !strings.Contains(body, `"first shot"`) {
		t.Errorf("body %s: a number stays unquoted and other fields stay as recorded", body)
	}
}

func TestImport_Given_a_value_that_only_looks_alike_Then_it_stays_literal(t *testing.T) {
	// Arrange: the page size 2 and the status 201 are small numbers that no id key produced.
	src := strings.Replace(recordedSession, `\"id\":90`, `\"id\":90,\"count\":2`, 1)

	// Act
	r, _ := Import([]byte(src), "pets.har")

	// Assert
	shots := nodeNamed(t, r.Flows[0].Nodes, "post-shots")
	if got := withValue(shots, "page"); got != "<none>" {
		t.Errorf("page was wired to %q", got)
	}
}

func TestImport_Given_a_group_with_several_calls_Then_only_path_ids_are_wired(t *testing.T) {
	// Arrange: the same endpoint is called twice, so a shared request cannot hold one call's wire.
	second := `,{"_resourceType": "fetch", "request": {"method": "POST", "url": "https://api.example.com/v1/pets/7/shots?owner=zzzz9999", "headers": [], "queryString": [{"name": "owner", "value": "zzzz9999"}]},
  "response": {"status": 201, "headers": [], "content": {"mimeType": "application/json", "text": "{\"id\":91}"}}}`
	src := strings.Replace(recordedSession, "]}}", second+"]}}", 1)

	// Act
	r, _ := Import([]byte(src), "pets.har")

	// Assert
	shots := nodeNamed(t, r.Flows[0].Nodes, "post-shots")
	if got := withValue(shots, "owner"); got != "<none>" {
		t.Errorf("owner = %q: a shared request keeps its literal query", got)
	}
	if got := withValue(shots, "id"); got != "{{nodes.post-pets.body.id}}" {
		t.Errorf("id = %q: path ids are still wired", got)
	}
}
