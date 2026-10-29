// SPDX-License-Identifier: Apache-2.0

package analysis

import "testing"

func tokenlessSchema() Contract {
	schema := map[string]any{"type": "object", "properties": map[string]any{"user": map[string]any{"type": "object"}}}
	return Contract{Source: SourceSchema, Samples: []Sample{{Source: SourceSchema, Status: 200, Schema: schema}}}
}

func TestReplaced_Given_a_changed_schema_for_a_version_Then_consumers_are_checked_against_it(t *testing.T) {
	// Arrange
	w := newWorkspaceCopy(t)
	src := Replaced{Base: w.contracts(), By: map[string]Contract{ContractKey("auth/login", "2"): tokenlessSchema()}}

	// Act
	got, _, err := Impact(w.load(), Target{Collection: identityID, Request: loginID}, "2", src)

	// Assert
	signup, _ := consumerStatus(got, nodeIn("signup", "login"))
	if err != nil || signup != Broken || got.Status != Broken || got.Versions[0].Contract.Source != SourceSchema {
		t.Fatalf("err %v signup %s report %+v", err, signup, got)
	}
}

func TestReplaced_Given_a_version_the_spec_removed_Then_every_consumer_is_broken_even_one_that_reads_nothing(t *testing.T) {
	// Arrange
	w := newWorkspaceCopy(t)
	src := Replaced{Base: w.contracts(), By: map[string]Contract{ContractKey("auth/login", "2"): {Removed: true}}}

	// Act
	got, _, err := Impact(w.load(), Target{Collection: identityID, Request: loginID}, "2", src)

	// Assert
	if err != nil || got.Status != Broken {
		t.Fatalf("err %v report %+v", err, got)
	}
	for _, c := range got.Versions[0].Consumers {
		if c.Status != Broken || c.Reason == "" {
			t.Errorf("consumer %+v should be broken with a reason", c)
		}
	}
}

func TestReplaced_Given_a_version_the_spec_does_not_mention_Then_the_base_contract_stands(t *testing.T) {
	// Arrange
	w := newWorkspaceCopy(t)
	src := Replaced{Base: w.contracts(), By: map[string]Contract{ContractKey("auth/other", "2"): {Removed: true}}}

	// Act
	got, _, err := Impact(w.load(), Target{Collection: identityID, Request: loginID}, "2", src)

	// Assert
	signup, _ := consumerStatus(got, nodeIn("signup", "login"))
	if err != nil || signup != Compatible || got.Versions[0].Contract.Source != SourceExample {
		t.Fatalf("err %v signup %s source %s", err, signup, got.Versions[0].Contract.Source)
	}
}
