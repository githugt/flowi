// SPDX-License-Identifier: Apache-2.0

package bundle

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/report"
)

func sample() Bundle {
	return Bundle{
		Manifest: Manifest{Flowi: "9.9", Collection: "Tools", FlowPath: "checkout", FlowName: "Checkout", Env: "staging", Seed: 42,
			Inputs: map[string]string{"user": "ada"}, Status: "failed", At: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Secrets: []string{"password"}},
		Report:      report.Run{ID: "R1", Status: "failed", Nodes: []report.Node{{ID: "login", Status: "passed"}}},
		Records:     []engine.Record{{Status: 201, Request: engine.SentRequest{Method: "POST", URL: "https://api.test/login"}}},
		Files:       map[string][]byte{"flowi.yaml": []byte("format: 1\n"), "flows/checkout.arazzo.yaml": []byte("arazzo: 1.0.1\n")},
		Environment: []byte("id: X\nname: staging\n"),
	}
}

func TestBundle_Given_a_bundle_When_written_and_read_Then_everything_comes_back(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	// Act
	err := Write(&buf, sample())
	got, readErr := Read(buf.Bytes())

	// Assert
	if err != nil || readErr != nil {
		t.Fatalf("write %v read %v", err, readErr)
	}
	want := sample()
	if got.Manifest.Seed != 42 || got.Manifest.Format != Format || got.Manifest.Inputs["user"] != "ada" || got.Manifest.Secrets[0] != "password" {
		t.Errorf("manifest %+v", got.Manifest)
	}
	if got.Report.ID != "R1" || len(got.Records) != 1 || got.Records[0].Status != 201 {
		t.Errorf("report %+v records %+v", got.Report, got.Records)
	}
	if string(got.Files["flows/checkout.arazzo.yaml"]) != string(want.Files["flows/checkout.arazzo.yaml"]) || string(got.Environment) != string(want.Environment) {
		t.Errorf("files %v env %q", got.Files, got.Environment)
	}
}

func TestRead_Given_an_entry_that_climbs_out_of_the_folder_Then_it_is_refused(t *testing.T) {
	for _, name := range []string{"collection/../evil.yaml", "collection//abs", "collection/a/../../b", "/collection/x"} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, f := range []string{"bundle.json", "report.json", "records.json"} {
				w, _ := zw.Create(f)
				_, _ = w.Write([]byte(map[string]string{"bundle.json": `{"format":1}`, "report.json": `{}`, "records.json": `[]`}[f]))
			}
			w, _ := zw.Create(name)
			_, _ = w.Write([]byte("x"))
			_ = zw.Close()

			_, err := Read(buf.Bytes())

			if err == nil || !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestRead_Given_a_newer_format_or_a_missing_part_Then_it_says_what_is_wrong(t *testing.T) {
	var newer bytes.Buffer
	zw := zip.NewWriter(&newer)
	w, _ := zw.Create("bundle.json")
	_, _ = w.Write([]byte(`{"format": 99}`))
	_ = zw.Close()

	_, errNewer := Read(newer.Bytes())
	_, errJunk := Read([]byte("not a zip"))

	if errNewer == nil || !strings.Contains(errNewer.Error(), "newer") {
		t.Errorf("newer: %v", errNewer)
	}
	if errJunk == nil || !strings.Contains(errJunk.Error(), "not a Flowi run bundle") {
		t.Errorf("junk: %v", errJunk)
	}
}

func TestRead_Given_an_entry_that_expands_past_the_limit_Then_it_is_refused(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("bundle.json")
	_, _ = w.Write([]byte(`{"format":1}`))
	big, _ := zw.Create("collection/big.txt")
	_, _ = big.Write(make([]byte, maxEntry+1))
	_ = zw.Close()

	_, err := Read(buf.Bytes())

	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err %v", err)
	}
}
