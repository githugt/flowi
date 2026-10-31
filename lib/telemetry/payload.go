// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrNotInCatalogue is returned for an event, field or value the catalogue
// does not name.
var ErrNotInCatalogue = errors.New("telemetry: not in the catalogue")

// payloadSchema is the payload format version.
const payloadSchema = 1

func knownOS() []string { return []string{"darwin", "linux", "windows", "other"} }

func knownArch() []string { return []string{"amd64", "arm64", "other"} }

// versionPattern admits release and development versions ("1.4.0",
// "dev+0123abcdef01") and nothing path- or URL-like.
var versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]{0,39}$`)

// Client describes the running program.
type Client struct {
	App     string `json:"app"` // cli | gui
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Sanitised returns c with every value squeezed into its closed set.
func (c Client) Sanitised() Client {
	if c.App != "cli" && c.App != "gui" {
		c.App = "cli"
	}
	if !versionPattern.MatchString(c.Version) {
		c.Version = "unknown"
	}
	if !contains(knownOS(), c.OS) {
		c.OS = "other"
	}
	if !contains(knownArch(), c.Arch) {
		c.Arch = "other"
	}
	return c
}

// Event is one reported occurrence.
type Event struct {
	Name   string            `json:"name"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Payload is everything one report contains.
type Payload struct {
	Schema int     `json:"schema"`
	Date   string  `json:"date"` // YYYY-MM-DD, UTC; no time of day
	Level  string  `json:"level"`
	Client Client  `json:"client"`
	Events []Event `json:"events"`
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Validate reports whether p contains only what the catalogue allows.
func (p Payload) Validate() error {
	if p.Schema != payloadSchema {
		return fmt.Errorf("%w: schema %d", ErrNotInCatalogue, p.Schema)
	}
	if !datePattern.MatchString(p.Date) || ParseLevel(p.Level) == LevelOff || p.Client != p.Client.Sanitised() {
		return fmt.Errorf("%w: header", ErrNotInCatalogue)
	}
	for _, e := range p.Events {
		if err := checkEvent(e.Name, e.Fields); err != nil {
			return err
		}
	}
	return nil
}

// Sample is an example payload for level, with example values, for
// `flowi telemetry show` and the GUI dialog. At LevelOff it has no events.
func Sample(level Level, c Client) Payload {
	p := Payload{Schema: payloadSchema, Date: "2026-01-31", Level: level.String(), Client: c.Sanitised(), Events: []Event{}}
	for _, s := range Catalogue() {
		if level == LevelOff || s.Level > level {
			continue
		}
		e := Event{Name: s.Name}
		if len(s.Fields) > 0 {
			e.Fields = map[string]string{}
		}
		for _, f := range s.Fields {
			e.Fields[f.Name] = f.Example
		}
		p.Events = append(p.Events, e)
	}
	return p
}
