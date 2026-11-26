// SPDX-License-Identifier: Apache-2.0

// Package graphqlx reads just enough of a GraphQL document to run it: which operations it holds, of which kind,
// and which one a request selects. It does not validate the document (the server does, and says so in its
// errors list); it only tells a query from a mutation from a subscription, so Flowi can refuse a GET mutation,
// pick the transport of a subscription and label a request.
package graphqlx

import (
	"errors"
	"fmt"
	"strings"
)

// Kind is the type of an operation.
type Kind string

// The kinds of operation.
const (
	Query        Kind = "query"
	Mutation     Kind = "mutation"
	Subscription Kind = "subscription"
)

// Operation is one operation definition of a document.
type Operation struct {
	Kind Kind
	// Name is empty for an anonymous operation.
	Name string
}

// Errors of Select.
var (
	// ErrNoOperation means the document defines no operation (it may hold only fragments, or be empty).
	ErrNoOperation = errors.New("the document defines no operation")
	// ErrAmbiguous means the document defines several operations and none was named.
	ErrAmbiguous = errors.New("the document defines several operations: name the one to run")
)

// Operations lists the operation definitions of doc in order. Fragments and everything inside brackets are skipped;
// comments, strings and block strings never count.
func Operations(doc string) []Operation {
	s := scanner{src: doc}
	var ops []Operation
	for {
		tok, ok := s.next()
		if !ok {
			return ops
		}
		switch {
		case s.top && tok == "{" && !s.wasHeader:
			ops = append(ops, Operation{Kind: Query}) // the shorthand: an anonymous query
		case s.top && s.wasHeader && s.kind != "" && s.nameWanted && isName(tok):
			ops[len(ops)-1].Name = tok
			s.nameWanted = false
		case s.top && !s.wasHeader && isKind(tok):
			ops = append(ops, Operation{Kind: Kind(tok)})
			s.header, s.kind, s.nameWanted = true, Kind(tok), true
		case s.top && !s.wasHeader && tok == "fragment":
			s.header, s.kind, s.nameWanted = true, "", false
		default:
			if s.wasHeader {
				s.nameWanted = false // anything after the keyword other than a name ends the search for one
			}
		}
	}
}

// Select returns the operation a request runs: the one called name, or the only one when name is empty.
func Select(doc, name string) (Operation, error) {
	ops := Operations(doc)
	if len(ops) == 0 {
		return Operation{}, ErrNoOperation
	}
	if name == "" {
		if len(ops) > 1 {
			return Operation{}, ErrAmbiguous
		}
		return ops[0], nil
	}
	for _, op := range ops {
		if op.Name == name {
			return op, nil
		}
	}
	return Operation{}, fmt.Errorf("the document defines no operation called %q (it has %s)", name, describe(ops))
}

func describe(ops []Operation) string {
	var names []string
	for _, op := range ops {
		if op.Name != "" {
			names = append(names, op.Name)
		}
	}
	if len(names) == 0 {
		return "none with a name"
	}
	return strings.Join(names, ", ")
}

func isKind(tok string) bool {
	return tok == string(Query) || tok == string(Mutation) || tok == string(Subscription)
}

// isName reports whether tok is a GraphQL name: a letter or underscore, then letters, digits and underscores.
func isName(tok string) bool {
	for i, r := range tok {
		letter := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !letter && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return tok != ""
}
