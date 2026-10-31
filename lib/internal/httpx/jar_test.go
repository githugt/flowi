// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func names(cs []*http.Cookie) map[string]string {
	out := map[string]string{}
	for _, c := range cs {
		out[c.Name] = c.Value
	}
	return out
}

func TestJar_should_send_cookie_set_by_previous_response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			return
		}
		c, err := r.Cookie("session")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(c.Value))
	}))
	defer srv.Close()
	o := defaultOptions()
	o.Jar = NewJar()
	c := newClient(t, o)

	get(t, c, srv.URL+"/set")
	a := get(t, c, srv.URL+"/echo")

	if a.Err != nil || string(a.Response.Body) != "abc" {
		t.Fatalf("got %+v", a)
	}
}

func TestJar_Save_then_LoadJar_should_restore_cookies(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cookies", "jar.json")
	u := mustURL(t, "https://api.example.com/v1/login")
	j := NewJar()
	j.SetCookies(u, []*http.Cookie{
		{Name: "session", Value: "abc", Path: "/"},
		{Name: "pref", Value: "dark", Path: "/", MaxAge: 3600},
	})

	if err := j.Save(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadJar(file, time.Now())

	if err != nil {
		t.Fatal(err)
	}
	if got := names(loaded.Cookies(mustURL(t, "https://api.example.com/other"))); got["session"] != "abc" || got["pref"] != "dark" {
		t.Fatalf("restored %v", got)
	}
	if len(loaded.All()) != 2 {
		t.Fatalf("All %+v", loaded.All())
	}
}

func TestJar_SetCookies_should_replace_same_cookie_and_forget_deleted(t *testing.T) {
	u := mustURL(t, "https://api.example.com/")
	j := NewJar()

	j.SetCookies(u, []*http.Cookie{{Name: "a", Value: "1", Path: "/"}, {Name: "b", Value: "1", Path: "/"}})
	j.SetCookies(u, []*http.Cookie{{Name: "a", Value: "2", Path: "/"}, {Name: "b", Value: "", Path: "/", MaxAge: -1}})

	all := j.All()
	if len(all) != 1 || all[0].Cookie.Name != "a" || all[0].Cookie.Value != "2" {
		t.Fatalf("All %+v", all)
	}
}

func TestJar_Save_should_drop_expired_cookies(t *testing.T) {
	file := filepath.Join(t.TempDir(), "jar.json")
	u := mustURL(t, "https://api.example.com/")
	j := NewJar()
	j.SetCookies(u, []*http.Cookie{{Name: "short", Value: "x", Path: "/", MaxAge: 60}, {Name: "long", Value: "y", Path: "/", MaxAge: 3600}})
	j.now = func() time.Time { return time.Now().Add(10 * time.Minute) }

	if err := j.Save(file); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadJar(file, time.Now())

	if err != nil || len(loaded.All()) != 1 || loaded.All()[0].Cookie.Name != "long" {
		t.Fatalf("got %+v %v", loaded.All(), err)
	}
}

func TestLoadJar_Given_missing_file_Then_empty_jar(t *testing.T) {
	j, err := LoadJar(filepath.Join(t.TempDir(), "none.json"), time.Now())

	if err != nil || len(j.All()) != 0 {
		t.Fatalf("got %+v %v", j, err)
	}
}

func TestLoadJar_Given_corrupt_file_Then_error_naming_file(t *testing.T) {
	file := filepath.Join(t.TempDir(), "jar.json")
	_ = os.WriteFile(file, []byte("{not json"), 0o600)

	_, err := LoadJar(file, time.Now())

	if err == nil || !contains(err.Error(), file) {
		t.Fatalf("got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s[:len(sub)] == sub || contains(s[1:], sub)))
}

func TestJar_should_respect_domain_and_path_scoping_after_reload(t *testing.T) {
	file := filepath.Join(t.TempDir(), "jar.json")
	j := NewJar()
	j.SetCookies(mustURL(t, "https://api.example.com/admin/x"), []*http.Cookie{{Name: "admin", Value: "1", Path: "/admin"}})
	j.SetCookies(mustURL(t, "https://other.example.org/"), []*http.Cookie{{Name: "other", Value: "1", Path: "/"}})
	if err := j.Save(file); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadJar(file, time.Now())

	if err != nil {
		t.Fatal(err)
	}
	if got := names(loaded.Cookies(mustURL(t, "https://api.example.com/admin/y"))); got["admin"] != "1" || got["other"] != "" {
		t.Fatalf("admin path got %v", got)
	}
	if got := names(loaded.Cookies(mustURL(t, "https://api.example.com/public"))); len(got) != 0 {
		t.Fatalf("public path got %v", got)
	}
}

func TestJar_concurrent_SetCookies_and_Cookies_race_free(t *testing.T) {
	j := NewJar()
	u := mustURL(t, "https://api.example.com/")
	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			for range 50 {
				j.SetCookies(u, []*http.Cookie{{Name: "c", Value: string(rune('a' + i)), Path: "/"}})
				_ = j.Cookies(u)
				_ = j.All()
			}
		})
	}
	wg.Wait()

	if len(j.All()) != 1 {
		t.Fatalf("All %+v", j.All())
	}
}

func TestJar_Clone_should_copy_cookies_and_not_share_later_writes(t *testing.T) {
	u := mustURL(t, "http://api.test/")
	j := NewJar()
	j.SetCookies(u, []*http.Cookie{{Name: "sid", Value: "s-1"}})

	c := j.Clone()
	c.SetCookies(u, []*http.Cookie{{Name: "lane", Value: "x"}})
	j.SetCookies(u, []*http.Cookie{{Name: "parent", Value: "y"}})

	got := names(c.Cookies(u))
	if got["sid"] != "s-1" || got["lane"] != "x" || got["parent"] != "" {
		t.Fatalf("clone cookies %v", got)
	}
	if parent := names(j.Cookies(u)); parent["lane"] != "" || parent["sid"] != "s-1" {
		t.Fatalf("parent cookies %v", parent)
	}
}
