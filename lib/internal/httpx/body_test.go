// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func collectionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixtures", "doc.txt"), []byte("hello file"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFileBody_Given_path_escaping_root_Then_OutsideRootError(t *testing.T) {
	root := collectionRoot(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "fixtures", "link.txt")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../x", outside, "fixtures/link.txt", "fixtures/../../x"} {
		t.Run(rel, func(t *testing.T) {
			_, err := FileBody(root, rel)

			var oe *OutsideRootError
			if !errors.As(err, &oe) || oe.Path != rel {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestFileBody_Given_file_inside_root_Then_reopens_for_each_attempt(t *testing.T) {
	body, err := FileBody(collectionRoot(t), "fixtures/doc.txt")
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		r, n, ctype, err := body.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(r)
		_ = r.Close()

		if string(data) != "hello file" || n != 10 || ctype != "text/plain; charset=utf-8" {
			t.Fatalf("got %q %d %q", data, n, ctype)
		}
	}
}

func TestFileBody_Given_missing_file_Then_not_exist_error(t *testing.T) {
	_, err := FileBody(collectionRoot(t), "fixtures/none.txt")

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestMultipartBody_should_send_fields_and_files_with_content_types(t *testing.T) {
	type seen struct{ field, file, fileType, filename string }
	got := make(chan seen, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		f, h, err := r.FormFile("upload")
		if err != nil {
			t.Error(err)
			return
		}
		data, _ := io.ReadAll(f)
		got <- seen{r.FormValue("title"), string(data), h.Header.Get("Content-Type"), h.Filename}
	}))
	defer srv.Close()
	body, err := MultipartBody(collectionRoot(t), []Part{
		{Name: "title", Value: "Report"},
		{Name: "upload", File: "fixtures/doc.txt", ContentType: "application/x-custom"},
	})
	if err != nil {
		t.Fatal(err)
	}

	attempts, err := newClient(t, defaultOptions()).Do(context.Background(), Request{Method: http.MethodPost, URL: srv.URL, Header: http.Header{}, Body: body})

	if err != nil || attempts[0].Err != nil {
		t.Fatalf("%v %v", err, attempts[0].Err)
	}
	if s := <-got; s != (seen{"Report", "hello file", "application/x-custom", "doc.txt"}) {
		t.Fatalf("server saw %+v", s)
	}
}

func TestMultipartBody_Given_file_outside_root_Then_error_before_request(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	_, err := MultipartBody(collectionRoot(t), []Part{{Name: "f", File: "../../etc/passwd"}})

	var oe *OutsideRootError
	if !errors.As(err, &oe) || hits.Load() != 0 {
		t.Fatalf("got %v, hits %d", err, hits.Load())
	}
}

func TestFormBody_should_url_encode_fields_in_order(t *testing.T) {
	r, n, ctype, err := FormBody([]FormField{{Name: "b", Value: "x y"}, {Name: "a", Value: "&="}}).Open()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)

	if string(data) != "b=x+y&a=%26%3D" || n != int64(len(data)) || ctype != "application/x-www-form-urlencoded" {
		t.Fatalf("got %q %d %q", data, n, ctype)
	}
}
