// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"

	"github.com/githugt/flowi-lib/engine"
)

func TestWatchPrompts_should_open_the_page_of_a_sign_in_and_pass_every_event_on(t *testing.T) {
	// Arrange
	in := make(chan engine.Event, 3)
	in <- engine.RunStarted{}
	in <- engine.AuthPrompted{Kind: "open", URL: "https://idp.example/authorize?x=1"}
	in <- engine.AuthPrompted{Kind: "device", URL: "https://idp.example/verify", Code: "ABCD"}
	close(in)
	var opened []string

	// Act
	var got []engine.Event
	for ev := range watchPrompts(in, func(u string) error { opened = append(opened, u); return nil }) {
		got = append(got, ev)
	}

	// Assert
	if len(got) != 3 {
		t.Fatalf("%d events came through, want all 3", len(got))
	}
	if len(opened) != 1 || opened[0] != "https://idp.example/authorize?x=1" {
		t.Fatalf("opened %v: only a sign-in the person does in this browser is opened; a device code is entered elsewhere", opened)
	}
}

func TestOpenBrowser_should_refuse_an_address_that_is_not_http(t *testing.T) {
	for _, address := range []string{"file:///etc/passwd", "javascript:alert(1)", "ssh://host", "", "-n"} {
		if err := openBrowser(address); err == nil {
			t.Errorf("%q was handed to the system", address)
		}
	}
}
