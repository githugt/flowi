// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// seedJar saves a jar with two cookies for the first collection.
func seedJar(t *testing.T, w *Workspace) {
	t.Helper()
	jar := httpx.NewJar()
	u, _ := url.Parse("https://api.example.com/login")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "session", Value: "abc", Path: "/", HttpOnly: true},
		{Name: "theme", Value: "dark", Path: "/", Expires: time.Now().Add(time.Hour)},
	})
	file := filepath.Join(w.DataDir(w.Collections()[0].Manifest.ID), "cookies", "jar.json")
	if err := jar.Save(file); err != nil {
		t.Fatal(err)
	}
}

func TestCookies_should_list_the_saved_jar_of_a_collection(t *testing.T) {
	w := demoWorkspace(t)
	seedJar(t, w)

	got, err := w.Cookies("")

	if err != nil || len(got) != 2 || got[0].Name != "session" || got[0].Domain != "api.example.com" || !got[0].HTTPOnly || !got[0].Expires.IsZero() || got[1].Expires.IsZero() {
		t.Fatalf("cookies %+v err %v", got, err)
	}
}

func TestCookies_Given_no_jar_Then_none(t *testing.T) {
	w := demoWorkspace(t)

	got, err := w.Cookies("")

	if err != nil || len(got) != 0 {
		t.Fatalf("cookies %+v err %v", got, err)
	}
}

func TestDeleteCookie_should_remove_only_that_cookie(t *testing.T) {
	w := demoWorkspace(t)
	seedJar(t, w)
	before, _ := w.Cookies("")

	err := w.DeleteCookie("", before[0].Key)
	after, _ := w.Cookies("")

	if err != nil || len(after) != 1 || after[0].Name != "theme" {
		t.Fatalf("after %+v err %v", after, err)
	}
}

func TestDeleteCookie_Given_a_cookie_already_gone_Then_no_error(t *testing.T) {
	w := demoWorkspace(t)
	seedJar(t, w)

	if err := w.DeleteCookie("", "nope\x00x\x00/"); err != nil {
		t.Fatal(err)
	}
}
