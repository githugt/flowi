// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const diffBefore = "id: 01JA00000000000000000000D1\nnodes:\n  - id: first\n    type: log\n    name: First\n  - id: second\n    type: log\n    needs: [first]\n"
const diffAfter = "id: 01JA00000000000000000000D1\nnodes:\n  - id: first\n    type: log\n    name: Hello\n  - id: second\n    type: log\n    needs: [first]\n  - id: third\n    type: note\n"

func TestDiff_Given_an_earlier_file_Then_the_changes_are_listed_and_exit_code_follows_the_flag(t *testing.T) {
	// Before: no command showed what changed in a flow except raw YAML.
	e := newSendEnv(t)
	flow := filepath.Join(e.dir, "tools", "flows", "diffme.arazzo.yaml")
	older := filepath.Join(e.dir, "older.arazzo.yaml")
	writeFile(t, flow, arazzoFlow(diffBefore))
	writeFile(t, older, arazzoFlow(diffBefore))
	writeFile(t, flow, arazzoFlow(diffAfter))

	code, out, errOut := e.run(context.Background(), "diff", "diffme", "--against", older)
	exit, _, _ := e.run(context.Background(), "diff", "diffme", "--against", older, "--exit-code")

	if code != 0 || exit != 1 {
		t.Fatalf("exit %d and %d\n%s", code, exit, errOut)
	}
	for _, want := range []string{"step first renamed from First to Hello", "step third added (note)", "2 changes"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDiff_Given_nothing_changed_Then_it_says_so_and_exits_0_even_with_exit_code(t *testing.T) {
	e := newSendEnv(t)
	flow := filepath.Join(e.dir, "tools", "flows", "diffme.arazzo.yaml")
	writeFile(t, flow, arazzoFlow(diffBefore))

	code, out, errOut := e.run(context.Background(), "diff", "diffme", "--against", flow, "--exit-code")

	if code != 0 || !strings.Contains(out, "no changes") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestDiff_Given_bad_usage_Then_exit_2(t *testing.T) {
	e := newSendEnv(t)
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "diffme.arazzo.yaml"), arazzoFlow(diffBefore))
	tests := map[string][]string{
		"neither":       {"diff", "diffme"},
		"both":          {"diff", "diffme", "--rev", "HEAD", "--against", "x"},
		"option as rev": {"diff", "diffme", "--rev", "--output=x"},
		"missing file":  {"diff", "diffme", "--against", filepath.Join(e.dir, "nope")},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := e.run(context.Background(), args...)

			if code != 2 {
				t.Fatalf("exit %d\n%s", code, errOut)
			}
		})
	}
}

func TestDiff_Given_a_git_revision_Then_the_earlier_version_comes_from_git(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	e := newSendEnv(t)
	repo := filepath.Join(e.dir, "tools")
	flow := filepath.Join(repo, "flows", "diffme.arazzo.yaml")
	writeFile(t, flow, arazzoFlow(diffBefore))
	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null") //nolint:forbidigo // a test child process keeps the parent's PATH and HOME
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "before")
	writeFile(t, flow, arazzoFlow(diffAfter))

	code, out, errOut := e.run(context.Background(), "diff", "diffme", "--rev", "HEAD")

	if code != 0 || !strings.Contains(out, "step third added (note)") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}
