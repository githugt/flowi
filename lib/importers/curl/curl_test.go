// SPDX-License-Identifier: Apache-2.0

package curl

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/importers/importerstest"
	"github.com/githugt/flowi/lib/model"
)

func TestTokenize_should_follow_shell_quoting(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`curl 'a b' "c d" e\ f`, []string{"curl", "a b", "c d", "e f"}},
		{"curl a \\\n  b", []string{"curl", "a", "b"}},
		{`curl "say \"hi\" \$x \\ \q"`, []string{"curl", `say "hi" $x \ \q`}},
		{`curl $'a\nb\'c\x41é'`, []string{"curl", "a\nb'cAé"}},
		{`curl 'it''s'`, []string{"curl", "its"}},
		{`curl ""`, []string{"curl", ""}},
	}
	for _, tc := range cases {
		got, err := tokenize(tc.in, false)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !slices.Equal(texts(got), tc.want) {
			t.Errorf("tokenize(%q) = %q, want %q", tc.in, texts(got), tc.want)
		}
	}
}

func TestTokenize_cmd_should_undo_caret_escapes(t *testing.T) {
	in := "curl ^\"https://x/^\" ^\n  --data-raw ^\"^{^\\^\"id^\\^\":1^}^\""

	got, err := tokenize(in, true)

	if err != nil || !slices.Equal(texts(got), []string{"curl", "https://x/", "--data-raw", `{"id":1}`}) {
		t.Fatalf("got %q %v", texts(got), err)
	}
}

func TestTokenize_should_report_where_an_unclosed_quote_starts(t *testing.T) {
	_, err := tokenize("curl 'abc", false)

	var se *syntaxError
	if !errors.As(err, &se) || se.offset != 5 {
		t.Fatalf("err %v", err)
	}
}

func texts(ts []token) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.text
	}
	return out
}

func TestImport_corpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			r, fatal := Import(src, filepath.Base(file))

			importerstest.Golden(t, strings.TrimSuffix(filepath.Base(file), ".txt")+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_never_write_credentials(t *testing.T) {
	for _, name := range []string{"devtools-bash.txt", "devtools-cmd.txt", "multi.txt"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}

		out := importerstest.Render(t, mustImport(t, src, name), nil)

		for _, secret := range []string{"eyJhbGciOiJIUzI1NiJ9", "s3cr3t", "hunter2", "abc123"} {
			if strings.Contains(out, secret) {
				t.Errorf("%s: %q reached the result", name, secret)
			}
		}
	}
}

func mustImport(t *testing.T, src []byte, name string) importers.Result {
	t.Helper()
	r, fatal := Import(src, name)
	if len(fatal) > 0 {
		t.Fatalf("%s: %v", name, fatal)
	}
	return r
}

func FuzzImport(f *testing.F) {
	for _, seed := range []string{"curl 'a", `curl "a\`, "curl $'\\x", "curl ^\"a^", "curl -H", "curl -X", "curl --data", "curl \\", "curl $'\\u12'"} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, in string) {
		Import([]byte(in), "fuzz.txt") // must not panic
	})
}

// Before: credentials in the URL, query and body, and unknown flags'
// arguments, reached the request or the report.
func TestImport_should_not_leak_credentials_from_urls_bodies_or_flags(t *testing.T) {
	src := "curl 'https://alice:hunter2@api.example.com/v1/users?api_key=S1&accessToken=S2&page=1' -H 'X-AccessToken: S3'\n" +
		"curl https://api.example.com/v1/login --data-raw '{\"password\":\"S4\",\"user\":\"ada\"}'\n" +
		"curl https://api.example.com/v1/form -d 'user=ada' -d 'password=S5'\n" +
		"curl --oauth2-bearer S6 https://api.example.com/v1/me --proxy-user bob:S7 -x http://bob:S8@proxy\n"

	r, _ := Import([]byte(src), "paste.txt")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"hunter2", "S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
	if len(r.Requests) != 4 || r.Requests[3].Versions[0].URL != "https://api.example.com/v1/me" {
		t.Fatalf("requests %d, last URL %q", len(r.Requests), r.Requests[len(r.Requests)-1].Versions[0].URL)
	}
}

// Before: 20k commands took 21s (positions rescanned the source).
func TestImport_should_stay_fast_on_large_pastes(t *testing.T) {
	src := strings.Repeat("curl https://api.example.com/v1/a\n", 20000) + "curl 'https://x/" + strings.Repeat("\u200b", 20000) + "'\n"
	done := make(chan struct{})

	go func() {
		defer close(done)
		Import([]byte(src), "big.txt")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("import of a large paste did not finish in 5s")
	}
}

// File paths are literal text, not templates.
func TestImport_should_keep_file_paths_literal(t *testing.T) {
	r, _ := Import([]byte("curl https://x/u -F 'f=@{{secret.z}}.png'\n"), "p.txt")

	part := r.Requests[0].Versions[0].Body.Multipart[0].Value.([]model.Field)[0].Value
	if part != `{{ "{{" }}secret.z}}.png` {
		t.Fatalf("part %v", part)
	}
}
