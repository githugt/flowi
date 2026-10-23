// SPDX-License-Identifier: Apache-2.0

// Package migrate checks collection format versions and plans the steps
// that upgrade older files to the current format.
package migrate

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/githugt/flowi-lib/diag"
)

// CurrentFormat is the collection format this version of Flow-I writes.
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
