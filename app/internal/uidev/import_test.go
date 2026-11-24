// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev_test

import (
	"slices"
	"testing"

	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/uidev"
)

const socket = `asyncapi: 3.0.0
info: {title: Socket, version: 1.0.0}
servers: {local: {host: "localhost:8080", protocol: ws}}
channels: {echo: {address: /ws}}
operations:
  echo: {action: send, title: Echo, channel: {$ref: "#/channels/echo"}}
`

func TestBackend_PreviewImportSource_should_plan_the_document_it_is_given_and_ApplyImportReport_should_show_it_in_the_collection(t *testing.T) {
	b := uidev.New()
	t.Cleanup(func() { _ = b.Close() })
	src := flowi.ImportSource{Kind: flowi.ImportAsyncAPI, Name: flowi.PastedSourceName, Data: []byte(socket), Into: "demo/ws"}

	plan, err := b.PreviewImportSource(src, uidev.Collection().Manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	report, err := b.ApplyImportReport(plan, flowi.ApplyOptions{})

	// Before: every preview was the same canned plan, whatever was pasted.
	if err != nil || len(plan.Files) != 2 || plan.Files[1].Rel != "requests/demo/ws/echo.req.yaml" || plan.Files[1].Name != "Echo" || len(report.Written) != 2 {
		t.Fatalf("plan %+v report %+v err %v", plan.Files, report, err)
	}
	var paths []string
	for _, r := range b.Collections()[0].Requests {
		paths = append(paths, r.Path)
	}
	if !slices.Contains(paths, "demo/ws/echo") {
		t.Fatalf("the sidebar's collection lacks the imported request: %v", paths)
	}
	again, _ := b.PreviewImportSource(src, uidev.Collection().Manifest.ID)
	if !again.Files[1].Exists || !again.Files[1].Same {
		t.Fatalf("a second preview should find the request already there: %+v", again.Files[1])
	}
}
