// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"
)

// Level is how much the user allows to be reported.
type Level int

// Levels, in increasing order of what is reported.
const (
	LevelOff Level = iota
	LevelMinimal
	LevelFull
)

func (l Level) String() string {
	switch l {
	case LevelMinimal:
		return "minimal"
	case LevelFull:
		return "full"
	}
	return "off"
}

// ParseLevel reads the telemetry setting; anything unknown is off.
func ParseLevel(s string) Level {
	switch s {
	case "minimal":
		return LevelMinimal
	case "full":
		return LevelFull
	}
	return LevelOff
}

// Event names.
const (
	EventAppStarted  = "app_started"
	EventCommandRun  = "command_run"
	EventRunFinished = "run_finished"
)

// FieldSpec is one field of an event: its value must be one of Allowed.
type FieldSpec struct {
	Name    string
	Doc     string
	Allowed []string
	Example string
}

// EventSpec is one reportable event.
type EventSpec struct {
	Name   string
	Doc    string
	Level  Level // the lowest level that reports it
	Fields []FieldSpec
}

// CommandNames is the closed set of command names a command_run event may
// carry. The CLI maps any other command to "other".
func CommandNames() []string {
	return []string{
		"send", "run", "ls", "validate", "fmt", "serve", "import", "export", "new", "config", "secret", "env",
		"workspace", "init", "schedule", "news", "changelog", "telemetry", "version", "impact", "cookies",
		"blueprint", "other",
	}
}

func durationBuckets() []string { return []string{"<1s", "<10s", "<1m", "<10m", ">=10m"} }

func countBuckets() []string { return []string{"0", "1-5", "6-20", "21+"} }

// Catalogue lists every event that can ever be reported.
func Catalogue() []EventSpec {
	return []EventSpec{
		{Name: EventAppStarted, Level: LevelMinimal, Doc: "The application was started. It carries no fields; the version, system and architecture are in the payload's client block."},
		{Name: EventCommandRun, Level: LevelFull, Doc: "A command-line command finished.", Fields: []FieldSpec{
			{Name: "command", Doc: "Which command, never its arguments.", Allowed: CommandNames(), Example: "send"},
			{Name: "exit", Doc: "How it ended.", Allowed: []string{"ok", "failed", "usage", "cancelled", "error"}, Example: "ok"},
		}},
		{Name: EventRunFinished, Level: LevelFull, Doc: "A send or flow run finished.", Fields: []FieldSpec{
			{Name: "kind", Doc: "A single request or a whole flow.", Allowed: []string{"send", "flow"}, Example: "flow"},
			{Name: "status", Doc: "How the run ended.", Allowed: []string{"passed", "failed", "error", "cancelled", "aborted"}, Example: "passed"},
			{Name: "duration", Doc: "How long it took, as a range.", Allowed: durationBuckets(), Example: "<10s"},
			{Name: "nodes", Doc: "How many nodes ran, as a range.", Allowed: countBuckets(), Example: "6-20"},
		}},
	}
}

func specOf(name string) (EventSpec, bool) {
	for _, s := range Catalogue() {
		if s.Name == name {
			return s, true
		}
	}
	return EventSpec{}, false
}

// checkEvent reports whether name and fields are exactly what the
// catalogue describes.
func checkEvent(name string, fields map[string]string) error {
	spec, ok := specOf(name)
	if !ok {
		return fmt.Errorf("%w: event %q", ErrNotInCatalogue, name)
	}
	if len(fields) != len(spec.Fields) {
		return fmt.Errorf("%w: event %q has %d fields, the catalogue lists %d", ErrNotInCatalogue, name, len(fields), len(spec.Fields))
	}
	for _, f := range spec.Fields {
		v, present := fields[f.Name]
		if !present || !contains(f.Allowed, v) {
			// The offending value is not echoed: it may be what the catalogue exists to keep out.
			return fmt.Errorf("%w: field %q of event %q", ErrNotInCatalogue, f.Name, name)
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Reference renders the catalogue as the markdown reference page.
func Reference() string {
	var b strings.Builder
	b.WriteString("# Telemetry\n\n")
	b.WriteString("<!-- Generated from telemetry.Catalogue; run `go test ./telemetry -run Reference -update`. -->\n\n")
	b.WriteString(Explanation() + "\n\n")
	b.WriteString("## Events\n\n")
	for _, s := range Catalogue() {
		fmt.Fprintf(&b, "### `%s` (%s)\n\n%s\n\n", s.Name, s.Level, s.Doc)
		if len(s.Fields) == 0 {
			continue
		}
		b.WriteString("| Field | Meaning | Values |\n|---|---|---|\n")
		for _, f := range s.Fields {
			fmt.Fprintf(&b, "| `%s` | %s | `%s` |\n", f.Name, f.Doc, strings.Join(f.Allowed, "`, `"))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Client block\n\n| Field | Values |\n|---|---|\n")
	fmt.Fprintf(&b, "| `app` | `cli`, `gui` |\n| `version` | the release version, or `unknown` |\n| `os` | `%s` |\n| `arch` | `%s` |\n",
		strings.Join(knownOS(), "`, `"), strings.Join(knownArch(), "`, `"))
	return b.String()
}

// Explanation is the plain-language statement shown wherever the user
// chooses a level.
func Explanation() string {
	return "Telemetry is off unless you turn it on. It never contains URLs, request or response bodies, " +
		"names of requests, flows or collections, ids, file paths, environment names, variable values or secrets. " +
		"There is no install id, so reports cannot be tied together or to you.\n\n" +
		"- off: nothing is collected or sent.\n" +
		"- minimal: once per start, the app (cli or gui), its version, your operating system and CPU architecture, and the day.\n" +
		"- full: minimal, plus which command ran and how it ended, and for each run its kind, outcome, duration and node count as ranges.\n\n" +
		"Run `flowi telemetry show` to see exactly what would be sent. Nothing is sent unless the telemetry_endpoint setting is also set."
}
