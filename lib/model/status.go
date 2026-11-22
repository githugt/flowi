// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Status is a development status a request, flow or folder can carry: where the work stands. A collection has the
// default statuses and may add its own, or change the colour and icon of a default, in the statuses of its flowi.yaml.
type Status struct {
	// Name is how files refer to the status ("in-development") and what the person reads.
	Name string
	// Color is one of StatusColors; "" draws gray.
	Color string
	// Icon is one of StatusIcons; "" draws a flag.
	Icon string
}

// DefaultStatuses are the statuses every collection has, in the order a piece of work moves through them.
func DefaultStatuses() []Status {
	return []Status{
		{Name: "planning", Color: "gray", Icon: "planning"},
		{Name: "in-development", Color: "blue", Icon: "code"},
		{Name: "review", Color: "purple", Icon: "eye"},
		{Name: "testing", Color: "amber", Icon: "flask"},
		{Name: "production", Color: "green", Icon: "rocket"},
	}
}

// StatusColors are the colours a status may have. The window draws each from its own theme token, checked for contrast
// on every theme, so a status cannot be given a colour that is unreadable on one.
func StatusColors() []string {
	return []string{"gray", "blue", "sky", "teal", "green", "amber", "red", "pink", "purple"}
}

// StatusIcons are the icons a status may have, by the name the window draws them under. The window's own test
// checks that it has a drawing for each.
func StatusIcons() []string {
	return []string{
		"planning", "code", "eye", "flask", "rocket", "flag", "star", "bug", "bolt", "lock", "archive", "bookmark",
		"pause", "clock", "alert",
	}
}

const (
	defaultStatusColor = "gray"
	defaultStatusIcon  = "flag"
	maxStatusName      = 40
)

var statusName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CheckStatusName says why name cannot be a status name, or nil. A name is lower case so that "Review" and "review"
// are not two statuses, and has no space so it is as easy to type in a file and on a command line as a tag.
func CheckStatusName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("the name is empty")
	case len(name) > maxStatusName:
		return fmt.Errorf("the name is longer than %d characters", maxStatusName)
	case !statusName.MatchString(name):
		return fmt.Errorf("use lower-case letters, digits, '-' and '_' (no spaces), starting with a letter or digit")
	}
	return nil
}

// CheckStatus says what is wrong with a status definition, or nil: its name, a colour the window cannot draw, an icon
// it does not know.
func CheckStatus(s Status) error {
	if err := CheckStatusName(s.Name); err != nil {
		return err
	}
	if s.Color != "" && !slices.Contains(StatusColors(), s.Color) {
		return fmt.Errorf("%q is not a status colour (use %s)", s.Color, strings.Join(StatusColors(), ", "))
	}
	if s.Icon != "" && !slices.Contains(StatusIcons(), s.Icon) {
		return fmt.Errorf("%q is not a status icon (use %s)", s.Icon, strings.Join(StatusIcons(), ", "))
	}
	return nil
}

// Resolved fills the colour and icon a status leaves out with the defaults, so a drawing never has a blank.
func (s Status) Resolved() Status {
	if s.Color == "" {
		s.Color = defaultStatusColor
	}
	if s.Icon == "" {
		s.Icon = defaultStatusIcon
	}
	return s
}

// StatusList lists the statuses of the collection: the defaults in their order, then its own in the order the manifest
// gives them. A manifest status named like a default replaces that default's colour and icon in place, so a team can
// recolour "testing" without losing its place in the order. A status named twice keeps its first definition.
func (m Manifest) StatusList() []Status {
	out := DefaultStatuses()
	defaults := len(out)
	seen := map[string]bool{}
	for _, s := range m.Statuses {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		if i := slices.IndexFunc(out[:defaults], func(o Status) bool { return o.Name == s.Name }); i >= 0 {
			out[i] = s
			continue
		}
		out = append(out, s)
	}
	for i := range out {
		out[i] = out[i].Resolved()
	}
	return out
}

// StatusNamed finds a status of the collection by name.
func (c Collection) StatusNamed(name string) (Status, bool) {
	list := c.Manifest.StatusList()
	i := slices.IndexFunc(list, func(s Status) bool { return s.Name == name })
	if i < 0 {
		return Status{}, false
	}
	return list[i], true
}

// StatusOf is the status an item has and where it comes from.
type StatusOf struct {
	// Name is the status; "" when neither the item nor a folder above it sets one.
	Name string
	// From is "" when the item sets the status itself, otherwise the folder it is inherited from, as that folder's Dir
	// ("requests/auth").
	From string
}

// RequestStatus resolves the status of r: its own, else that of the nearest folder above it that sets one. Before,
// there was no status; a folder's applies to everything below it exactly as a folder's tags do, except that the
// nearest wins instead of all adding up, because an item has one status.
func (c Collection) RequestStatus(r Request) StatusOf {
	return c.statusAt(path.Join("requests", path.Dir(r.Path)), r.Status)
}

// FlowStatus resolves the status of f as RequestStatus does for a request.
func (c Collection) FlowStatus(f Flow) StatusOf {
	return c.statusAt(path.Join("flows", path.Dir(f.Path)), f.Status)
}

// FolderStatus resolves the status of the folder dir ("requests/auth"): its own, else the nearest folder above it.
func (c Collection) FolderStatus(dir string) StatusOf {
	for _, f := range c.Folders {
		if f.Dir == dir && f.Status != "" {
			return StatusOf{Name: f.Status}
		}
	}
	return c.statusAt(path.Dir(dir), "")
}

// statusAt walks up from dir (the folder an item sits in) to find the nearest folder with a status. own wins over all.
func (c Collection) statusAt(dir, own string) StatusOf {
	if own != "" {
		return StatusOf{Name: own}
	}
	for d := dir; d != "." && d != "/"; d = path.Dir(d) {
		if i := slices.IndexFunc(c.Folders, func(f Folder) bool { return f.Dir == d }); i >= 0 && c.Folders[i].Status != "" {
			return StatusOf{Name: c.Folders[i].Status, From: d}
		}
	}
	return StatusOf{}
}

// RequestsWithStatus lists the requests whose resolved status is name, in file order.
func (c Collection) RequestsWithStatus(name string) []Request {
	var out []Request
	for _, r := range c.Requests {
		if c.RequestStatus(r).Name == name {
			out = append(out, r)
		}
	}
	return out
}

// FlowsWithStatus lists the flows whose resolved status is name, in file order.
func (c Collection) FlowsWithStatus(name string) []Flow {
	var out []Flow
	for _, f := range c.Flows {
		if c.FlowStatus(f).Name == name {
			out = append(out, f)
		}
	}
	return out
}
