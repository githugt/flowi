// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	flowi "github.com/githugt/flowi/lib"
	"os"
	"path/filepath"
	"testing"
)

const sampleRunReport = `{"id":"r1","kind":"flow","status":"passed","seed":1,"nodes":[{"path":"get","id":"get","type":"request","status":"passed","attempts":[{"n":1,"method":"GET","url":"http://x","status":200}]}],"duration_ms":4}`

// Before: coverage knew only the saved history, which a CI job does not have, so its statuses were always unseen.
func TestReadRunReports_should_read_files_and_the_json_files_of_a_folder(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	one := write("one.json", sampleRunReport)
	write("two.json", sampleRunReport)
	write("notes.txt", "not a report")

	file, errFile := readRunReports([]string{one})
	folder, errFolder := readRunReports([]string{dir})

	if errFile != nil || errFolder != nil || len(file) != 1 || len(folder) != 2 {
		t.Fatalf("file %d (%v), folder %d (%v)", len(file), errFile, len(folder), errFolder)
	}
	if file[0].Nodes[0].Attempts[0].Status != 200 {
		t.Fatalf("the attempt's status was not read: %+v", file[0])
	}
}

func TestReadRunReports_Given_something_that_is_not_a_run_report_Then_it_is_a_usage_error_naming_it(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "other.json")
	if err := os.WriteFile(bad, []byte(`{"hello":"world"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{bad, filepath.Join(dir, "missing.json")} {
		_, err := readRunReports([]string{path})

		var usage *UsageError
		if !errors.As(err, &usage) {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestRunDrift_Given_request_without_update_Then_it_is_a_usage_error(t *testing.T) {
	err := runDrift(nil, nil, "flow", driftFlags{requests: []string{"a/b"}})

	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("--request alone must be refused: %v", err)
	}
}

func TestSchemasToCreate_should_count_only_the_responses_without_a_schema_among_the_accepted_requests(t *testing.T) {
	rep := flowi.DriftReport{Requests: []flowi.RequestDrift{
		{Request: "a/one", NoSchema: true}, {Request: "a/two", NoSchema: true}, {Request: "a/three"},
	}}

	all := schemasToCreate(rep, nil)
	one := schemasToCreate(rep, []string{"a/one", "a/three"})

	if all != 2 || one != 1 {
		t.Fatalf("all %d, one %d", all, one)
	}
}
