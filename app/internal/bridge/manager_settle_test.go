// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Before: Mount and its siblings sent the action and returned at once, so the screen cleared the typed folder whether or
// not it worked and the error was left at the top of the page.
func TestManagerService_Mount_Given_the_folder_is_refused_Then_the_error_comes_back_to_the_caller(t *testing.T) {
	b := &vmtest.Backend{MountErr: errors.New("no flowi.yaml in /nowhere")}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	err := app.Manager().Mount("/nowhere")

	if err == nil || !strings.Contains(err.Error(), "no flowi.yaml") || !b.Called("Mount /nowhere") {
		t.Fatalf("err %v calls %v", err, b.CallLog())
	}
}

func TestManagerService_Mount_Given_it_works_Then_no_error_and_the_workspace_is_reloaded(t *testing.T) {
	b := &vmtest.Backend{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	err := app.Manager().Mount("/good")
	created := app.Manager().NewCollection("Shop", "/shop")

	if err != nil || created != nil || !b.Called("InitCollection /shop Shop") {
		t.Fatalf("err %v created %v calls %v", err, created, b.CallLog())
	}
}

// Before: a new user had to find a server and a collection to try anything; the example opened by itself.
func TestManagerService_OpenExample_Given_the_backend_opens_it_Then_no_error_and_a_refusal_comes_back_to_the_caller(t *testing.T) {
	good := &vmtest.Backend{}
	appGood := bridge.NewApp(good, bridge.NewInlineActor(), (&published{}).emit)
	appGood.Start(t.Context())
	bad := &vmtest.Backend{Check: vmtest.Checks{Err: errors.New("no free port")}}
	appBad := bridge.NewApp(bad, bridge.NewInlineActor(), (&published{}).emit)
	appBad.Start(t.Context())

	okErr := appGood.Manager().OpenExample()
	badErr := appBad.Manager().OpenExample()

	if okErr != nil || !good.Called("OpenExample") || badErr == nil || !strings.Contains(badErr.Error(), "no free port") {
		t.Fatalf("ok %v bad %v calls %v", okErr, badErr, good.CallLog())
	}
}

// Before: a send stopped for sending a secret to a host could only be fixed from the command line or Settings.
func TestManagerService_AllowSecretHost_should_pass_the_host_on_and_return_a_refusal(t *testing.T) {
	good := &vmtest.Backend{}
	appGood := bridge.NewApp(good, bridge.NewInlineActor(), (&published{}).emit)
	appGood.Start(t.Context())
	bad := &vmtest.Backend{Check: vmtest.Checks{Err: errors.New(`"*" is not a host`)}}
	appBad := bridge.NewApp(bad, bridge.NewInlineActor(), (&published{}).emit)
	appBad.Start(t.Context())

	okErr := appGood.Manager().AllowSecretHost("api.example.com")
	badErr := appBad.Manager().AllowSecretHost("*")

	if okErr != nil || !good.Called("AllowSecretHost api.example.com") || badErr == nil || !strings.Contains(badErr.Error(), "is not a host") {
		t.Fatalf("ok %v bad %v calls %v", okErr, badErr, good.CallLog())
	}
}
