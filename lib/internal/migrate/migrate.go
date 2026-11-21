// SPDX-License-Identifier: Apache-2.0

// Package migrate checks collection format versions and plans the steps
// that upgrade older files to the current format.
package migrate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/diag"
)

// CurrentFormat is the collection format this version of Flowi writes.
const CurrentFormat = 1

// Step upgrades every file of a collection from format From to From+1.
// Files maps collection-relative paths to contents.
type Step struct {
	From     int
	Describe string
	Apply    func(files map[string][]byte) (map[string][]byte, error)
}

// Steps returns every known migration. Format 1 is the first format, so
// there are none yet.
func Steps() []Step { return nil }

// Check validates a manifest's format number. It returns the zero
// Diagnostic when the format is supported.
func Check(format int) diag.Diagnostic {
	switch {
	case format == 0:
		return diag.New(diag.CodeFormatMissing, diag.Location{Path: "$.format"}, nil)
	case format > CurrentFormat:
		return diag.New(diag.CodeFormatTooNew, diag.Location{Path: "$.format"},
			map[string]string{"got": strconv.Itoa(format), "max": strconv.Itoa(CurrentFormat)})
	}
	return diag.Diagnostic{}
}

// Plan returns the steps that move a collection from format `from` to
// CurrentFormat, in order.
func Plan(from int) ([]Step, error) {
	return planWith(Steps(), from, CurrentFormat)
}

func planWith(steps []Step, from, to int) ([]Step, error) {
	var out []Step
	for f := from; f < to; f++ {
		i := slices.IndexFunc(steps, func(s Step) bool { return s.From == f })
		if i < 0 {
			return nil, fmt.Errorf("no migration from format %d to %d", f, f+1)
		}
		out = append(out, steps[i])
	}
	return out, nil
}

// CheckVersion tells whether the running release is new enough for a collection that names min in its flowi.yaml
// (min_version). It returns the zero Diagnostic when it is, when min is empty, or when either is not a version
// number (a development build has none: it reads everything).
func CheckVersion(min, have string) diag.Diagnostic {
	need, ok1 := parseVersion(min)
	got, ok2 := parseVersion(have)
	if !ok1 || !ok2 || !need.after(got) {
		return diag.Diagnostic{}
	}
	return diag.New(diag.CodeFormatNeedsNewer, diag.Location{Path: "$.min_version"}, map[string]string{"min": min, "have": have})
}

type version struct {
	parts [3]int
	pre   string
}

// after says whether v is a later release than o (a pre-release comes before its release).
func (v version) after(o version) bool {
	for i := range v.parts {
		if v.parts[i] != o.parts[i] {
			return v.parts[i] > o.parts[i]
		}
	}
	return o.pre != "" && v.pre == ""
}

// parseVersion reads "1.4.0", "v1.4" or "1.4.0-rc.1"; a missing part is 0.
func parseVersion(s string) (version, bool) {
	var v version
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, v.pre, _ = strings.Cut(s, "-")
	fields := strings.Split(s, ".")
	if s == "" || len(fields) > 3 {
		return v, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return v, false
		}
		v.parts[i] = n
	}
	return v, true
}
