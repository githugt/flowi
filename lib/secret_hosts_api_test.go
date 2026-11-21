// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"
)

// Before: the only way to let a secret go to a host was to retype the whole settings list.
func TestAllowSecretHost_should_add_one_host_at_a_time_and_keep_the_others(t *testing.T) {
	w := demoWorkspace(t)

	a, errA := w.AllowSecretHost("API.Example.com")
	b, errB := w.AllowSecretHost("https://tools.example.org:8443/path?x=1")
	_, errC := w.AllowSecretHost("api.example.com") // again: nothing changes
	got := w.AllowedSecretHosts()

	if errA != nil || errB != nil || errC != nil || a != "api.example.com" || b != "tools.example.org:8443" {
		t.Fatalf("%q %v / %q %v / %v", a, errA, b, errB, errC)
	}
	if want := []string{"api.example.com", "tools.example.org:8443"}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAllowSecretHost_should_refuse_what_is_not_a_host_or_would_allow_every_host(t *testing.T) {
	w := demoWorkspace(t)

	for _, bad := range []string{"", "  ", "*", "*x.example", "a.*.example", "two words", "a.example,b.example", "user@host.example", "https://"} {
		if _, err := w.AllowSecretHost(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if got := w.AllowedSecretHosts(); len(got) != 0 {
		t.Errorf("list %v", got)
	}
	if got, err := w.AllowSecretHost("*.corp.example"); err != nil || got != "*.corp.example" {
		t.Errorf("wildcard: %q %v", got, err)
	}
}

func TestDenySecretHost_should_remove_one_host_and_say_when_it_was_not_there(t *testing.T) {
	w := demoWorkspace(t)
	_, _ = w.AllowSecretHost("a.example")
	_, _ = w.AllowSecretHost("b.example")

	removed, err := w.DenySecretHost("A.example")
	again, errAgain := w.DenySecretHost("a.example")

	if err != nil || errAgain != nil || !removed || again {
		t.Fatalf("removed %v %v, again %v %v", removed, err, again, errAgain)
	}
	if got := w.AllowedSecretHosts(); !slices.Equal(got, []string{"b.example"}) {
		t.Fatalf("got %v", got)
	}
}
