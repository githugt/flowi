// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/template"
)

// hintValueMax is how much of a long value a hint shows.
const hintValueMax = 80

// secretMask stands for a secret's value wherever it is shown: the value never leaves the vault for a tooltip.
const secretMask = "••••"

// VariableHint is what a {{variable}} chip says when hovered, and whether clicking it can set the variable.
type VariableHint struct {
	// Text is the sentence shown, such as "staging → https://api.example.com (from environment)".
	Text string
	// Known is false for a name no environment defines: the chip is marked and clicking it offers to set it.
	Known bool
	// Environment is the selected environment, which a click would add the variable to; empty when none is chosen.
	Environment string
}

// DescribeVariable says what the expression inside {{ }} is now: the value the selected environment gives it and
// which environment in the chain it comes from, "••••" for a secret (the value is never read), what a generator
// makes, or that it is known only when the request runs. A name that is not set says so, with the environment a
// click would set it in.
func (ws *Workspace) DescribeVariable(expr string) VariableHint {
	expr = strings.TrimSpace(expr)
	env := ws.SelectedEnv()
	unset := func(what string) VariableHint {
		if env == "" {
			return VariableHint{Text: what + " is not set: no environment is chosen."}
		}
		return VariableHint{Text: fmt.Sprintf("%s is not set in %s. Click to set it.", what, env), Environment: env}
	}
	if gen, ok := strings.CutPrefix(expr, "$"); ok {
		name, _, _ := strings.Cut(gen, "(")
		name = strings.TrimSpace(name)
		for _, g := range template.Generators() {
			if g.Name == name {
				return VariableHint{Text: fmt.Sprintf("$%s: %s A new value on every send.", name, strings.TrimSpace(g.Description)), Known: true}
			}
		}
		return VariableHint{Text: "$" + name + " is not a generator."}
	}
	if !isPlainName(expr) {
		return VariableHint{Text: "An expression: worked out when the request runs.", Known: true}
	}
	root, rest := splitRoot(expr)
	switch {
	case slices.Contains(runtimeNamespaces(), root):
		return VariableHint{Text: root + ": known only when the request runs.", Known: true}
	case root == "secret":
		name := firstSegment(rest)
		if _, from, ok := ws.chainVar(name, true); ok {
			return VariableHint{Text: fmt.Sprintf("%s → %s (secret%s)", name, secretMask, fromNote(from, env)), Known: true}
		}
		return unset("Secret " + firstSegment(rest))
	}
	name := root
	if root == "env" {
		name = firstSegment(rest)
	}
	if v, from, ok := ws.chainVar(name, false); ok {
		return VariableHint{Text: fmt.Sprintf("%s → %s (from %s)", name, shorten(v), fromWords(from, env)), Known: true}
	}
	return unset(name)
}

// chainVar finds name in the selected environment or the ones it builds on, the nearest first. secret picks
// whether it must be a secret: the nearest environment decides, as in VariableNames.
func (ws *Workspace) chainVar(name string, secret bool) (value, from string, ok bool) {
	chain := ws.SelectedChain()
	for i := len(chain) - 1; i >= 0; i-- {
		for _, e := range ws.snap.environment {
			if e.Name != chain[i] {
				continue
			}
			for _, v := range e.Vars {
				if v.Name == name {
					if v.Secret != secret {
						return "", "", false
					}
					return v.Value, e.Name, true
				}
			}
		}
	}
	return "", "", false
}

func fromWords(from, selected string) string {
	if from == selected {
		return "environment " + from
	}
	return "environment " + from + ", which " + selected + " builds on"
}

func fromNote(from, selected string) string {
	if from == selected {
		return " of " + from
	}
	return " of " + from + ", which " + selected + " builds on"
}

// shorten cuts a long value for a tooltip, on a character boundary.
func shorten(v string) string {
	if v == "" {
		return "(empty)"
	}
	if utf8.RuneCountInString(v) <= hintValueMax {
		return v
	}
	return string([]rune(v)[:hintValueMax]) + "…"
}
