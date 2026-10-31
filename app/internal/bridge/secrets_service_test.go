// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/githugt/flowi/internal/bridge"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func TestSecretsService_Reveal_should_return_one_value_and_log_that_it_happened_without_the_value(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	b := &vmtest.Backend{SecretValues: map[string]string{"local/password": "s3cr3t-pw"}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit, bridge.WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	app.Start(t.Context())

	got, err := app.Secrets().Reveal("local", "password")

	if err != nil || got != "s3cr3t-pw" {
		t.Fatalf("got %q %v", got, err)
	}
	if !strings.Contains(logs.String(), "SecretRevealed") || strings.Contains(logs.String(), "s3cr3t-pw") {
		t.Fatalf("log %q", logs.String())
	}
}

func TestSecretsService_Reveal_Given_no_value_Then_an_error_and_no_log_of_a_reveal(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), (&published{}).emit, bridge.WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	app.Start(t.Context())

	got, err := app.Secrets().Reveal("local", "password")

	if err == nil || got != "" || strings.Contains(logs.String(), "SecretRevealed") {
		t.Fatalf("got %q %v log %q", got, err, logs.String())
	}
}
