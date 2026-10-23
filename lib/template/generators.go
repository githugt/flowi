// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	mrand "math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// Generator is one $name generator. Description and Args drive the
// generated reference docs.
type Generator struct {
	Name        string
	Description string
	Args        string // human signature, e.g. "(min, max)"; "" when it takes none
	Fn          func(f *gofakeit.Faker, now time.Time, args []any) (any, error)
}

const (
	defaultIntMin = 0
	defaultIntMax = 100
)

// Generators returns every generator, sorted by name.
func Generators() []Generator {
	noArgs := func(fn func(f *gofakeit.Faker) any) func(*gofakeit.Faker, time.Time, []any) (any, error) {
		return func(f *gofakeit.Faker, _ time.Time, _ []any) (any, error) { return fn(f), nil }
	}
	gens := []Generator{
		{Name: "uuid-v4", Description: "Random UUID (version 4).", Fn: noArgs(func(f *gofakeit.Faker) any { return f.UUID() })},
		{Name: "first-name", Description: "Person's first name.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.FirstName() })},
		{Name: "last-name", Description: "Person's last name.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.LastName() })},
		{Name: "full-name", Description: "First and last name.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.Name() })},
		{Name: "email", Description: "Email address.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.Email() })},
		{Name: "city-name", Description: "City name.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.City() })},
		{Name: "phone", Description: "Phone number (digits only).", Fn: noArgs(func(f *gofakeit.Faker) any { return f.Phone() })},
		{Name: "word", Description: "Random English word.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.Word() })},
		{Name: "bool", Description: "true or false.", Fn: noArgs(func(f *gofakeit.Faker) any { return f.Bool() })},
		{Name: "int", Description: "Whole number between min and max, inclusive (default 0–100).", Args: "(min, max)", Fn: genInt},
		{Name: "float", Description: "Decimal number between min and max.", Args: "(min, max)", Fn: genFloat},
		{Name: "timestamp", Description: "Current time as Unix seconds.", Fn: func(_ *gofakeit.Faker, now time.Time, _ []any) (any, error) {
			return now.Unix(), nil
		}},
		{Name: "now", Description: "Current time; layout is a Go layout or RFC3339, RFC3339Nano, date, time (default RFC3339).", Args: "(layout)", Fn: genNow},
	}
	slices.SortFunc(gens, func(a, b Generator) int { return strings.Compare(a.Name, b.Name) })
	return gens
}

func genInt(f *gofakeit.Faker, _ time.Time, args []any) (any, error) {
	lo, hi, err := intRange(args)
	if err != nil {
		return nil, fmt.Errorf("$int(min, max): %w", err)
	}
	return f.IntRange(lo, hi), nil
}

func intRange(args []any) (int, int, error) {
	if len(args) == 0 {
		return defaultIntMin, defaultIntMax, nil
	}
	if len(args) != 2 {
		return 0, 0, fmt.Errorf("want 2 numbers, got %d arguments", len(args))
	}
	lo, okLo := args[0].(int)
	hi, okHi := args[1].(int)
	if !okLo || !okHi || lo > hi {
		return 0, 0, fmt.Errorf("want whole numbers with min <= max, got %v, %v", args[0], args[1])
	}
	return lo, hi, nil
}

func genFloat(f *gofakeit.Faker, _ time.Time, args []any) (any, error) {
	if len(args) == 0 {
		return f.Float64Range(defaultIntMin, defaultIntMax), nil
	}
	lo, okLo := toFloat(args[0])
	hi, okHi := 0.0, false
	if len(args) == 2 {
		hi, okHi = toFloat(args[1])
	}
	if !okLo || !okHi || lo > hi {
		return nil, fmt.Errorf("$float(min, max): want 2 numbers with min <= max, got %v", args)
	}
	return f.Float64Range(lo, hi), nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func genNow(_ *gofakeit.Faker, now time.Time, args []any) (any, error) {
	layouts := map[string]string{
		"RFC3339": time.RFC3339, "RFC3339Nano": time.RFC3339Nano, "date": time.DateOnly, "time": time.TimeOnly,
	}
	layout := time.RFC3339
	if len(args) > 1 {
		return nil, fmt.Errorf("$now(layout): want at most 1 argument, got %d", len(args))
	}
	if len(args) == 1 {
		name, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("$now(layout): layout must be text, got %v", args[0])
		}
		layout = name
		if named, isNamed := layouts[name]; isNamed {
			layout = named
		}
	}
	return now.Format(layout), nil
}

// Generation produces generator values for one run from one seed, so a
// run's generated data can be reproduced. Not safe for concurrent use.
type Generation struct {
	faker  *gofakeit.Faker
	now    func() time.Time
	byName map[string]Generator
}

// NewGeneration starts a generation from seed; now supplies the clock.
func NewGeneration(seed uint64, now func() time.Time) *Generation {
	byName := map[string]Generator{}
	for _, g := range Generators() {
		byName[g.Name] = g
	}
	return &Generation{faker: gofakeit.New(seed), now: now, byName: byName}
}

// Func returns the GeneratorFunc for a Scope.
func (g *Generation) Func() GeneratorFunc {
	return func(name string, args []any) (any, error) {
		gen, ok := g.byName[name]
		if !ok {
			return nil, &UnknownGeneratorError{Name: name}
		}
		return gen.Fn(g.faker, g.now(), args)
	}
}

// NewSeed returns a random non-zero seed for runs that were given none; it
// is reported so the run can be repeated.
func NewSeed() uint64 {
	for {
		// Seeds pick fake test data, not keys: predictability is harmless.
		if seed := mrand.Uint64(); seed != 0 { //nolint:gosec // see above
			return seed
		}
	}
}
