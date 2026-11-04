// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func writeReadOnlyFlow(t *testing.T, e *sendEnv) {
	t.Helper()
	// /slow holds the connection for 30s by default, so the flow reads the instant config endpoint instead.
	writeFile(t, filepath.Join(e.dir, "tools", "requests", "config.req.yaml"), "id: 01JA00000000000000000000T4\nname: Config\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/v1/config\"\n")
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "readonly.arazzo.yaml"), `arazzo: 1.1.0
info:
  title: Read only
  version: 1.0.0
workflows:
  - workflowId: readonly
    summary: Read only
    steps: []
    x-flowi-flow:
      id: 01JA00000000000000000000T8
      name: Read only
      nodes:
        - id: config
          type: request
          request:
            id: 01JA00000000000000000000T4
            path: config
x-flowi-version: 1
`)
}

func TestWatch_should_record_the_first_pass_then_report_no_change(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)

	code1, out1, errOut := e.run(context.Background(), "watch", "readonly", "--env", "local")
	code2, out2, _ := e.run(context.Background(), "watch", "readonly", "--env", "local")

	if code1 != 0 || !strings.Contains(out1, "First pass") {
		t.Fatalf("first: exit %d\n%s\n%s", code1, out1, errOut)
	}
	if code2 != 0 || !strings.Contains(out2, "No change") {
		t.Fatalf("second: exit %d\n%s", code2, out2)
	}
}

func TestWatch_Given_a_flow_that_posts_Then_is_refused_with_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "watch", "user-creation", "--env", "local")

	if code != 2 || !strings.Contains(errOut, "POST") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}

func TestWatchAdd_should_save_a_watching_schedule_that_both_listings_show(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)

	code, out, errOut := e.run(context.Background(), "watch", "add", "contract", "--flow", "readonly", "--cron", "@hourly", "--env", "local")
	_, watchLs, _ := e.run(context.Background(), "watch", "ls")
	_, scheduleLs, _ := e.run(context.Background(), "schedule", "ls", "--output", "json")

	if code != 0 || !strings.Contains(out, "contract") {
		t.Fatalf("add: exit %d\n%s\n%s", code, out, errOut)
	}
	if !strings.Contains(watchLs, "contract") || !strings.Contains(watchLs, "readonly") {
		t.Fatalf("watch ls:\n%s", watchLs)
	}
	if !strings.Contains(scheduleLs, `"watch":true`) {
		t.Fatalf("schedule ls should mark it a watch:\n%s", scheduleLs)
	}
}

func TestWatchAdd_Given_a_flow_that_posts_Then_is_refused_with_the_reason(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "watch", "add", "bad", "--flow", "user-creation", "--cron", "@hourly", "--env", "local")
	_, ls, _ := e.run(context.Background(), "schedule", "ls")

	if code != 2 || !strings.Contains(errOut, "POST") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	if strings.Contains(ls, "bad") {
		t.Fatalf("saved anyway:\n%s", ls)
	}
}

func TestWatchLs_should_leave_out_schedules_that_do_not_watch(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)
	e.run(context.Background(), "schedule", "add", "nightly", "--flow", "readonly", "--cron", "@daily", "--env", "local")
	e.run(context.Background(), "watch", "add", "contract", "--flow", "readonly", "--cron", "@hourly", "--env", "local")

	_, out, _ := e.run(context.Background(), "watch", "ls")

	if strings.Contains(out, "nightly") || !strings.Contains(out, "contract") {
		t.Fatalf("watch ls:\n%s", out)
	}
}

func TestWatchRm_should_remove_a_watch_and_refuse_a_schedule_that_is_not_one(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)
	e.run(context.Background(), "schedule", "add", "nightly", "--flow", "readonly", "--cron", "@daily", "--env", "local")
	e.run(context.Background(), "watch", "add", "contract", "--flow", "readonly", "--cron", "@hourly", "--env", "local")

	removed, _, _ := e.run(context.Background(), "watch", "rm", "contract")
	refused, _, errOut := e.run(context.Background(), "watch", "rm", "nightly")
	_, ls, _ := e.run(context.Background(), "schedule", "ls")

	if removed != 0 || refused != 2 || !strings.Contains(errOut, "not a watch") {
		t.Fatalf("removed %d, refused %d\n%s", removed, refused, errOut)
	}
	if strings.Contains(ls, "contract") || !strings.Contains(ls, "nightly") {
		t.Fatalf("schedule ls:\n%s", ls)
	}
}

func TestWatchRun_should_do_one_pass_like_the_bare_command(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)

	code, out, errOut := e.run(context.Background(), "watch", "run", "readonly", "--env", "local")

	if code != 0 || !strings.Contains(out, "First pass") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestScheduleAdd_Given_watch_Then_saves_a_watching_schedule(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)

	code, _, errOut := e.run(context.Background(), "schedule", "add", "contract", "--flow", "readonly", "--cron", "@hourly", "--env", "local", "--watch")
	_, out, _ := e.run(context.Background(), "watch", "ls")

	if code != 0 || !strings.Contains(out, "contract") {
		t.Fatalf("exit %d\n%s\n%s", code, errOut, out)
	}
}

func TestWatchLs_should_say_how_the_last_pass_went(t *testing.T) {
	e := newSendEnv(t)
	writeReadOnlyFlow(t, e)
	e.run(context.Background(), "watch", "add", "contract", "--flow", "readonly", "--cron", "@hourly", "--env", "local")
	_, before, _ := e.run(context.Background(), "watch", "ls")

	e.run(context.Background(), "watch", "run", "readonly", "--env", "local")
	_, after, _ := e.run(context.Background(), "watch", "ls")

	if !strings.Contains(before, "not yet") || !strings.Contains(after, "ok") {
		t.Fatalf("before:\n%s\nafter:\n%s", before, after)
	}
}
