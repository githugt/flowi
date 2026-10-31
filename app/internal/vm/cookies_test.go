// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	flowi "github.com/githugt/flowi-lib"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newCookies(b *vmtest.Backend) (*vm.Cookies, *vm.Worker) {
	w := vmtest.SyncWorker()
	return vm.NewCookies(b, w, "c1"), w
}

func TestCookies_Load_should_list_the_jar(t *testing.T) {
	b := sampleBackend()
	b.CookieJar = []flowi.CookieInfo{{Key: "a", Name: "session"}, {Key: "b", Name: "theme"}}
	c, w := newCookies(b)

	c.Load()
	w.Wait()

	if len(c.Rows()) != 2 || c.Rows()[0].Name != "session" || c.Err() != "" {
		t.Fatalf("rows %+v err %q", c.Rows(), c.Err())
	}
}

func TestCookies_Delete_should_remove_the_cookie_and_show_the_rest(t *testing.T) {
	b := sampleBackend()
	b.CookieJar = []flowi.CookieInfo{{Key: "a", Name: "session"}, {Key: "b", Name: "theme"}}
	c, w := newCookies(b)
	c.Load()
	w.Wait()

	c.Delete("a")
	w.Wait()

	if len(c.Rows()) != 1 || c.Rows()[0].Name != "theme" || !b.Called("DeleteCookie a") {
		t.Fatalf("rows %+v", c.Rows())
	}
}

func TestCookies_Clear_should_empty_the_jar(t *testing.T) {
	b := sampleBackend()
	b.CookieJar = []flowi.CookieInfo{{Key: "a"}}
	c, w := newCookies(b)
	c.Load()
	w.Wait()

	c.Clear()
	w.Wait()

	if !b.Called("ClearCookies c1") {
		t.Fatal("jar not cleared")
	}
}

func TestCookies_Load_should_show_a_failure_instead_of_the_rows(t *testing.T) {
	b := sampleBackend()
	b.CookiesErr = errors.New("jar damaged")
	c, w := newCookies(b)

	c.Load()
	w.Wait()

	if c.Err() == "" || len(c.Rows()) != 0 {
		t.Fatalf("err %q rows %+v", c.Err(), c.Rows())
	}
}
