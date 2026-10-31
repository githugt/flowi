// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactServer_should_hide_what_a_connection_string_holds_in_an_error(t *testing.T) {
	server := "postgres://app-user:s3cr3t-pw@db.internal.example:5433/ledger?sslmode=disable"
	cause := errors.New(`failed to connect to user=app-user database=ledger: db.internal.example:5433 (10.1.2.3): password "s3cr3t-pw" is wrong`)

	got := redactServer(cause, server).Error()

	for _, hidden := range []string{"app-user", "s3cr3t-pw", "ledger", "db.internal.example", "5433"} {
		if strings.Contains(got, hidden) {
			t.Errorf("%q is in %q", hidden, got)
		}
	}
	if !strings.Contains(got, "failed to connect") || !strings.Contains(got, "is wrong") {
		t.Errorf("the rest of the message was lost: %q", got)
	}
}

func TestRedactServer_should_leave_the_cause_to_errors_Is(t *testing.T) {
	cause := errors.New("down")

	got := redactServer(cause, "mysql://u:p@h/db")

	if !errors.Is(got, cause) {
		t.Fatal("the cause is hidden from errors.Is")
	}
}

func TestRedactServer_should_not_mangle_a_message_for_short_parts(t *testing.T) {
	got := redactServer(errors.New("a connection to db failed at 5"), "mysql://a:b@c:5/d").Error()

	if got != "a connection to db failed at 5" {
		t.Fatalf("got %q", got)
	}
}

func TestRedactServer_Given_a_server_that_is_not_a_url_should_keep_the_message(t *testing.T) {
	got := redactServer(errors.New("unable to open database file"), "shop.db").Error()

	if got != "unable to open database file" {
		t.Fatalf("got %q", got)
	}
}

func TestRedactServer_should_hide_a_whole_key_value_connection_string(t *testing.T) {
	server := "host=db.private user=reader password=pw-1234567 dbname=books"

	got := redactServer(errors.New("could not use "+server), server).Error()

	if strings.Contains(got, "pw-1234567") || strings.Contains(got, "db.private") {
		t.Fatalf("got %q", got)
	}
}
