// SPDX-License-Identifier: Apache-2.0

package graphqlx_test

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/lib/graphqlx"
)

func TestOperations_should_list_the_operations_of_a_document(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []graphqlx.Operation
	}{
		{"shorthand query", `{ user { name } }`, []graphqlx.Operation{{Kind: graphqlx.Query}}},
		{"named query with variables", `query GetUser($id: ID!) { user(id: $id) { name } }`, []graphqlx.Operation{{Kind: graphqlx.Query, Name: "GetUser"}}},
		{"anonymous mutation", `mutation { like(id: 1) }`, []graphqlx.Operation{{Kind: graphqlx.Mutation}}},
		{"subscription", `subscription OnLike { liked { id } }`, []graphqlx.Operation{{Kind: graphqlx.Subscription, Name: "OnLike"}}},
		{"two operations and a fragment", `
			# query Commented { a }
			query A { ...F }
			fragment F on User { name }
			mutation B($s: String = "query X { y }") { set(s: $s) }`,
			[]graphqlx.Operation{{Kind: graphqlx.Query, Name: "A"}, {Kind: graphqlx.Mutation, Name: "B"}}},
		{"keyword inside a string and a block string", `query A { f(a: "mutation M { x }", b: """subscription S { y }""") }`,
			[]graphqlx.Operation{{Kind: graphqlx.Query, Name: "A"}}},
		{"a field called query inside a selection", `{ query { id } }`, []graphqlx.Operation{{Kind: graphqlx.Query}}},
		{"only a fragment", `fragment F on User { name }`, nil},
		{"empty", ``, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := graphqlx.Operations(tc.doc)

			if len(got) != len(tc.want) {
				t.Fatalf("Operations(%q) = %v, want %v", tc.doc, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("operation %d = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSelect_should_pick_by_name_or_the_only_operation(t *testing.T) {
	doc := `query A { a } mutation B { b }`

	byName, errName := graphqlx.Select(doc, "B")
	_, errAmbiguous := graphqlx.Select(doc, "")
	_, errMissing := graphqlx.Select(doc, "C")
	_, errNone := graphqlx.Select("fragment F on T { a }", "")
	only, errOnly := graphqlx.Select(`{ a }`, "")

	if errName != nil || byName.Kind != graphqlx.Mutation {
		t.Errorf("Select by name = %v, %v", byName, errName)
	}
	if !errors.Is(errAmbiguous, graphqlx.ErrAmbiguous) {
		t.Errorf("no name with two operations: %v, want ErrAmbiguous", errAmbiguous)
	}
	if errMissing == nil {
		t.Error("an unknown name must be an error")
	}
	if !errors.Is(errNone, graphqlx.ErrNoOperation) {
		t.Errorf("a document with no operation: %v, want ErrNoOperation", errNone)
	}
	if errOnly != nil || only.Kind != graphqlx.Query {
		t.Errorf("the only operation = %v, %v", only, errOnly)
	}
}

func FuzzOperations(f *testing.F) {
	for _, seed := range []string{`{ a }`, `query A($x: Int) { a(x: $x) }`, `"""`, `query "`, `{{{{`, `}}}}`, `# only a comment`} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, doc string) {
		graphqlx.Operations(doc) // must not panic or loop
	})
}
