// SPDX-License-Identifier: AGPL-3.0-or-later

// Package palette reads the palette file of the "custom" theme: a JSON file
// that names the colours of the window by role and, optionally, a rainbow of
// tints for the collection rows. What the file does not name falls back to the
// base theme (dark or light); a file that cannot be a readable theme is
// refused with the reason, so the window keeps the theme it has.
package palette

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxFile bounds a palette file: it is a few lines.
const maxFile = 64 << 10

// minContrast is the least contrast (WCAG) text must have against the background.
const minContrast = 4.5

// Palette is a custom theme: the base it starts from, the CSS variables it
// sets, and the rainbow of collection tints.
type Palette struct {
	Base    string            // "dark" or "light"
	Vars    map[string]string // CSS variable → #rrggbb
	Rainbow []string
}

// roles maps a colour role of the file to the window's CSS variable.
func roles() map[string]string {
	return map[string]string{
		"background": "--bg", "surface": "--panel", "line": "--line", "foreground": "--text", "muted": "--muted",
		"primary": "--accent", "on_primary": "--on-accent", "danger": "--fail", "success": "--ok", "warning": "--warn",
	}
}

// baseText and baseBackground are the base themes' text and background, used
// when a file names only one of the two.
func baseColours(base string) (text, background string) {
	if base == "light" {
		return "#1c2230", "#f5f6f8"
	}
	return "#e6e9ef", "#0f1116"
}

var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type file struct {
	Base    string            `json:"base"`
	Colors  map[string]string `json:"colors"`
	Rainbow []string          `json:"rainbow"`
}

// Load reads and checks the palette file at path.
func Load(path string) (Palette, error) {
	data, err := read(path)
	if err != nil {
		return Palette{}, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return Palette{}, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if f.Base == "" {
		f.Base = "dark"
	}
	if f.Base != "dark" && f.Base != "light" {
		return Palette{}, fmt.Errorf(`the palette's base is "dark" or "light", not %q`, f.Base)
	}
	p := Palette{Base: f.Base, Vars: map[string]string{}}
	if err := p.setColours(f.Colors); err != nil {
		return Palette{}, err
	}
	if p.Rainbow, err = hexList("rainbow", f.Rainbow); err != nil {
		return Palette{}, err
	}
	if err := p.checkContrast(); err != nil {
		return Palette{}, err
	}
	return p, nil
}

func read(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading the palette file %s: %w", path, err)
	}
	if info.Size() > maxFile {
		return nil, fmt.Errorf("the palette file %s is larger than %d KiB", path, maxFile>>10)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the palette file %s: %w", path, err)
	}
	return data, nil
}

func (p *Palette) setColours(colours map[string]string) error {
	known := roles()
	for role, value := range colours {
		variable, ok := known[role]
		if !ok {
			names := make([]string, 0, len(known))
			for name := range known {
				names = append(names, name)
			}
			slices.Sort(names)
			return fmt.Errorf("%q is not a colour of the palette (they are %s)", role, strings.Join(names, ", "))
		}
		hex, err := normalise(value)
		if err != nil {
			return fmt.Errorf("the colour %s: %w", role, err)
		}
		p.Vars[variable] = hex
	}
	return nil
}

func hexList(name string, values []string) ([]string, error) {
	var out []string
	for i, v := range values {
		hex, err := normalise(v)
		if err != nil {
			return nil, fmt.Errorf("%s colour %d: %w", name, i+1, err)
		}
		out = append(out, hex)
	}
	return out, nil
}

// normalise is a colour as #rrggbb; only hex is accepted, so no text of the
// file can reach the window's style as anything else.
func normalise(v string) (string, error) {
	v = strings.TrimSpace(v)
	if !hexColour.MatchString(v) {
		return "", fmt.Errorf("%q is not a hex colour like #1e1e2e", v)
	}
	if len(v) == 4 {
		v = "#" + strings.Repeat(string(v[1]), 2) + strings.Repeat(string(v[2]), 2) + strings.Repeat(string(v[3]), 2)
	}
	return strings.ToLower(v), nil
}

// checkContrast refuses a palette whose text cannot be read on its background.
// A colour the file does not name is the base theme's.
func (p Palette) checkContrast() error {
	text, background := baseColours(p.Base)
	if v, ok := p.Vars["--text"]; ok {
		text = v
	}
	if v, ok := p.Vars["--bg"]; ok {
		background = v
	}
	if c := Contrast(text, background); c < minContrast {
		return fmt.Errorf("the text colour %s on the background %s has a contrast of %.1f, under the %.1f needed to read it", text, background, c, minContrast)
	}
	return nil
}

// Contrast is the WCAG contrast ratio of two #rrggbb colours (1 to 21).
func Contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(hex string) float64 {
	channel := func(from int) float64 {
		n, err := strconv.ParseUint(hex[from:from+2], 16, 8)
		if err != nil {
			return 0
		}
		c := float64(n) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(1) + 0.7152*channel(3) + 0.0722*channel(5)
}
