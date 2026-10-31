// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestSend_should_lay_a_flowi_overlay_over_the_version(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/overlay"))
	in := f.input(r)
	in.Overlay = &Overlay{Name: "vip", Ops: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}}

	// Act
	res, _ := f.send(in)

	// Assert
	if res.Status != StatusPassed || res.Record == nil {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	if got := echoedHeader(t, res.Record, "X-Tier"); got != "vip" {
		t.Errorf("the server saw X-Tier %q", got)
	}
	if res.Record.Overlay != "vip" {
		t.Errorf("record overlay = %q", res.Record.Overlay)
	}
}

func TestSend_should_warn_about_what_the_overlay_cannot_do_and_send_anyway(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/overlay"))
	in := f.input(r)
	in.Overlay = &Overlay{Name: "vip", Ops: []model.OverlayOp{
		{Verb: model.OverlayRemove, Path: "headers.X-Missing"},
		{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"},
	}}

	// Act
	res, _ := f.send(in)

	// Assert
	if res.Status != StatusPassed || !hasCode(res.Diags, diag.CodeOverlayNotFitting) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
	if echoedHeader(t, res.Record, "X-Tier") != "vip" {
		t.Error("the part that fits should still apply")
	}
}

func TestSend_should_keep_a_responses_per_overlay(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/overlay"))
	plain, vip := f.input(r), f.input(r)
	plain.Save, vip.Save = "last", "last"
	vip.Overlay = &Overlay{Name: "vip"}

	// Act
	f.send(plain)
	f.send(vip)

	// Assert
	dir := filepath.Join(f.deps().DataDir(f.coll().Manifest.ID), "responses", string(r.ID))
	for _, want := range []string{ResponseKey("1", ""), ResponseKey("1", "vip")} {
		if _, err := os.Stat(filepath.Join(dir, want, "last.json")); err != nil {
			t.Errorf("no saved response in %s: %v", want, err)
		}
	}
}

func TestResponseKey_should_leave_a_plain_version_alone(t *testing.T) {
	if ResponseKey("2.1", "") != "2.1" || ResponseKey("2.1", "vip") != "2.1--vip" {
		t.Error("unexpected response keys")
	}
}

func TestRequestNode_should_lay_its_flowi_overlay_over_the_version(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/"))
	f.update(func(c *model.Collection) {
		c.Overlays = []model.Overlay{{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}}}
	})
	n := requestNode("call", r)
	n.Request.Overlay = "vip"
	fl := f.addFlow("overlay", n)

	// Act
	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	call, _ := finishedNode(events, "call")
	if res.Status != StatusPassed || call.Outputs["status"] != 200 {
		t.Fatalf("result %+v call %+v", res, call)
	}
	if got := f.lastEcho()["X-Tier"]; got != "vip" {
		t.Errorf("the server saw X-Tier %q", got)
	}
}

func TestRequestNode_should_fail_when_its_flowi_overlay_does_not_exist(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/"))
	n := requestNode("call", r)
	n.Request.Overlay = "ghost"
	fl := f.addFlow("overlay", n)

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status == StatusPassed || !hasCode(res.Diags, diag.CodeOverlayNotFound) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestRun_Given_a_run_level_overlay_Then_a_step_that_names_none_is_sent_with_it(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/"))
	f.update(func(c *model.Collection) {
		c.Overlays = []model.Overlay{{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}}}
	})
	fl := f.addFlow("overlay", requestNode("call", r))
	in := f.runInput(fl)
	in.Overlay = "vip"

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), in)

	// Assert
	if res.Status != StatusPassed || f.lastEcho()["X-Tier"] != "vip" {
		t.Fatalf("status %v diags %v saw %q", res.Status, res.Diags, f.lastEcho()["X-Tier"])
	}
}

func TestRun_Given_a_run_level_overlay_and_a_step_with_its_own_Then_the_steps_own_wins(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/"))
	f.update(func(c *model.Collection) {
		c.Overlays = []model.Overlay{
			{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}},
			{Name: "guest", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "guest"}}},
		}
	})
	n := requestNode("call", r)
	n.Request.Overlay = "guest"
	fl := f.addFlow("overlay", n)
	in := f.runInput(fl)
	in.Overlay = "vip"

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), in)

	// Assert
	if res.Status != StatusPassed || f.lastEcho()["X-Tier"] != "guest" {
		t.Fatalf("status %v saw %q", res.Status, f.lastEcho()["X-Tier"])
	}
}
