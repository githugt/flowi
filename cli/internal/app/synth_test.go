// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// newSynthEnv mounts a collection with a create and a read request whose responses are known from schemas.
func newSynthEnv(t *testing.T) *sendEnv {
	t.Helper()
	e := newSendEnv(t)
	root := filepath.Join(e.dir, "pets")
	writeFile(t, filepath.Join(root, "flowi.yaml"), "format: 1\nid: 01JA00000000000000000000P1\nname: Pets\n")
	writeFile(t, filepath.Join(root, "requests", "create.req.yaml"), "id: 01JA00000000000000000000P2\nname: Create\nversions:\n  \"1\":\n    method: POST\n    url: \"{{env.base_url}}/pets\"\n")
	writeFile(t, filepath.Join(root, "requests", "create.schemas", "1-201.json"), `{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"}}}`)
	writeFile(t, filepath.Join(root, "requests", "read.req.yaml"), "id: 01JA00000000000000000000P3\nname: Read\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/pets/{{petId}}\"\n")
	if code, _, errOut := e.run(context.Background(), "workspace", "mount", root); code != 0 {
		t.Fatalf("mount: %d %s", code, errOut)
	}
	return e
}

func TestSynth_should_propose_a_create_then_read_flow_and_write_nothing_by_default(t *testing.T) {
	e := newSynthEnv(t)

	code, out, errOut := e.run(context.Background(), "synth", "--collection", "Pets")

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"pets", "create", "POST", "petId from create.id"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if _, listing, _ := e.run(context.Background(), "ls"); strings.Contains(out, "saved as") || strings.Contains(listing, "suggested") {
		t.Errorf("nothing should have been saved:\n%s\n%s", out, listing)
	}
}

func TestSynth_Given_write_Then_saves_the_flow_and_a_second_run_makes_another(t *testing.T) {
	e := newSynthEnv(t)

	code, out, errOut := e.run(context.Background(), "synth", "--collection", "Pets", "--write", "--output", "json")

	var got struct {
		Saved []string `json:"saved"`
	}
	if err := json.Unmarshal([]byte(out), &got); code != 0 || err != nil || len(got.Saved) != 1 {
		t.Fatalf("exit %d, %v\n%s\n%s", code, err, out, errOut)
	}
	_, again, _ := e.run(context.Background(), "synth", "--collection", "Pets", "--write", "--output", "json")
	if !strings.Contains(again, "pets-2") {
		t.Fatalf("a second write must not overwrite the first:\n%s", again)
	}
}

func TestSynth_Given_a_confidence_no_link_reaches_Then_proposes_nothing(t *testing.T) {
	e := newSynthEnv(t)

	code, out, _ := e.run(context.Background(), "synth", "--collection", "Pets", "--min-confidence", "1")

	if code != 0 || !strings.Contains(out, "Nothing to suggest") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestSynth_Given_a_confidence_out_of_range_Then_is_a_usage_error(t *testing.T) {
	e := newSynthEnv(t)

	code, _, _ := e.run(context.Background(), "synth", "--collection", "Pets", "--min-confidence", "2")

	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

// newNegativesEnv mounts a collection with one request that posts a JSON body with a bearer token.
func newNegativesEnv(t *testing.T) *sendEnv {
	t.Helper()
	e := newSendEnv(t)
	root := filepath.Join(e.dir, "users")
	writeFile(t, filepath.Join(root, "flowi.yaml"), "format: 1\nid: 01JA00000000000000000000W1\nname: Users\n")
	writeFile(t, filepath.Join(root, "requests", "create.req.yaml"), `id: 01JA00000000000000000000W2
name: Create
versions:
  "1":
    method: POST
    url: "{{env.base_url}}/users"
    headers:
      Authorization: "Bearer {{env.token}}"
    body:
      json: "{\"email\": \"a@b.c\", \"age\": 3}"
`)
	if code, _, errOut := e.run(context.Background(), "workspace", "mount", root); code != 0 {
		t.Fatalf("mount: %d %s", code, errOut)
	}
	return e
}

func TestSynth_Given_negatives_Then_it_lists_the_ways_to_send_the_request_wrong_and_writes_nothing(t *testing.T) {
	e := newNegativesEnv(t)

	code, out, errOut := e.run(context.Background(), "synth", "--negatives", "create", "--collection", "Users")

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"no-email", "bad-age", "stale-token", "--write"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	_, list, _ := e.run(context.Background(), "overlay", "ls")
	if strings.Contains(list, "no-email") {
		t.Fatalf("nothing should be written without --write:\n%s", list)
	}
}

func TestSynth_Given_negatives_and_write_Then_the_overlays_are_saved_and_run_with_send(t *testing.T) {
	e := newNegativesEnv(t)

	code, out, errOut := e.run(context.Background(), "synth", "--negatives", "create", "--collection", "Users", "--write")

	if code != 0 || !strings.Contains(out, "Saved 5 overlays") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	_, list, _ := e.run(context.Background(), "overlay", "ls")
	if !strings.Contains(list, "stale-token") {
		t.Fatalf("the overlays are missing:\n%s", list)
	}
	_, again, _ := e.run(context.Background(), "synth", "--negatives", "create", "--collection", "Users", "--write")
	if !strings.Contains(again, "Saved 0 overlays") || !strings.Contains(again, "Left alone") {
		t.Fatalf("a second run should leave what exists alone:\n%s", again)
	}
}

func TestSynth_Given_negatives_for_an_unknown_request_Then_it_fails(t *testing.T) {
	e := newNegativesEnv(t)

	code, _, errOut := e.run(context.Background(), "synth", "--negatives", "nope", "--collection", "Users")

	if code == 0 || errOut == "" {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}
