// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/theory/jsonpath"

	"github.com/githugt/flowi-lib/internal/jsonschema"
)

// rank orders statuses: the worst one wins.
func rank(s Status) int {
	return slices.Index([]Status{Compatible, Unknown, Broken}, s)
}

func worst(a, b Status) Status {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// checked sets every read's status against the contract, and the
// consumer's to the worst of them. A consumer that reads nothing is
// compatible: nothing it relies on can be missing.
func checked(c Consumer, contract Contract) Consumer {
	c.Reads = slices.Clone(c.Reads)
	c.Status = Compatible
	for i, r := range c.Reads {
		c.Reads[i] = checkRead(r, contract)
		c.Status = worst(c.Status, c.Reads[i].Status)
	}
	return c
}

// checkRead judges one read (design §9 and plan Decision 4): against the
// 2xx samples if there are any, else all; compatible if any sample has it,
// unknown if none has it but some cannot tell, broken otherwise. Reads
// already judged (unfilled inputs, unknown extract names) are kept.
func checkRead(r Read, contract Contract) Read {
	if r.Status != "" {
		return r
	}
	samples := relevantSamples(contract.Samples)
	if len(samples) == 0 {
		r.Status, r.Reason = Unknown, "no contract data"
		return r
	}
	r.Status = Broken
	for _, s := range samples {
		switch checkSample(r, s) {
		case Compatible:
			r.Status, r.Reason = Compatible, ""
			return r
		case Unknown:
			r.Status = Unknown
		}
	}
	verb := "not in "
	if r.Status == Unknown {
		verb = "cannot tell from "
	}
	r.Reason = verb + sourceName(contract.Source, samples)
	return r
}

func relevantSamples(all []Sample) []Sample {
	ok := slices.DeleteFunc(slices.Clone(all), func(s Sample) bool { return s.Status < 200 || s.Status > 299 })
	if len(ok) > 0 {
		return ok
	}
	return all
}

// sourceName describes samples for messages: "example 200.json",
// "examples 200.json, 201.json", "schema 1-200.json" or "the last saved
// response".
func sourceName(source string, samples []Sample) string {
	if source != SourceExample && source != SourceSchema {
		return "the last saved response"
	}
	var names []string
	for _, s := range samples {
		names = append(names, filepath.Base(s.File))
	}
	if len(names) == 1 {
		return source + " " + names[0]
	}
	return source + "s " + strings.Join(names, ", ")
}

// checkSample judges one read against one sample (plan Decision 5).
func checkSample(r Read, s Sample) Status {
	if s.Schema != nil {
		return checkSchema(r, s.Schema)
	}
	switch r.Kind {
	case ReadBody:
		if s.Body == nil {
			return Broken // a text or empty body has no fields
		}
		return walk(s.Body, r.steps)
	case ReadJSONPath:
		return checkJSONPath(r.Target, s)
	case ReadHeader:
		return checkHeader(r.Target, s)
	case ReadCookie:
		return checkCookie(r.Target, s)
	case ReadRegex:
		return checkRegex(r.Target, s)
	}
	return Compatible
}

// walk follows steps through a JSON value. Samples show shape, not
// length, so an index past the end checks element 0.
func walk(v any, steps []string) Status {
	for _, step := range steps {
		switch x := v.(type) {
		case map[string]any:
			next, ok := x[step]
			if !ok {
				return Broken
			}
			v = next
		case []any:
			i, err := strconv.Atoi(step)
			switch {
			case err != nil || i < 0:
				return Broken
			case len(x) == 0:
				return Unknown
			case i >= len(x):
				i = 0
			}
			v = x[i]
		case nil:
			return Unknown // null here may be an object elsewhere
		default:
			return Broken
		}
	}
	return Compatible
}

func checkJSONPath(path string, s Sample) Status {
	p, err := jsonpath.Parse(path)
	if err != nil {
		return Unknown // validate reports invalid paths
	}
	if s.Body == nil {
		return Broken
	}
	if len(p.Select(s.Body)) == 0 {
		return Broken
	}
	return Compatible
}

func checkHeader(name string, s Sample) Status {
	if !s.HeadersKnown {
		return Unknown
	}
	if len(s.Headers.Values(name)) == 0 {
		return Broken
	}
	return Compatible
}

// checkCookie looks only at the sample's Set-Cookie headers. At run time
// an extract may also find the cookie in the jar from an earlier
// response; a contract judges what this API returns.
func checkCookie(name string, s Sample) Status {
	if !s.HeadersKnown {
		return Unknown
	}
	for _, c := range (&http.Response{Header: s.Headers}).Cookies() {
		if c.Name == name {
			return Compatible
		}
	}
	return Broken
}

// checkRegex runs the regex over the body text, or the JSON body
// re-encoded when only JSON was recorded.
func checkRegex(pattern string, s Sample) Status {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Unknown // validate reports invalid patterns
	}
	text := s.Text
	if s.Body != nil {
		data, err := json.Marshal(s.Body)
		if err != nil {
			return Unknown
		}
		text = string(data)
	}
	if re.MatchString(text) {
		return Compatible
	}
	return Broken
}

// checkSchema judges a read against a response schema. Schemas describe
// bodies only: header, cookie and regex reads are unknown, as are
// JSONPaths beyond plain member and index steps.
func checkSchema(r Read, schema any) Status {
	var steps []string
	switch r.Kind {
	case ReadBody:
		steps = r.steps
	case ReadJSONPath:
		var ok bool
		if steps, ok = plainSteps(r.Target); !ok {
			return Unknown
		}
	default:
		return Unknown
	}
	switch jsonschema.Lookup(schema, steps) {
	case jsonschema.Present:
		return Compatible
	case jsonschema.Absent:
		return Broken
	}
	return Unknown
}

var plainStep = regexp.MustCompile(`^\.([A-Za-z_][A-Za-z0-9_]*)|^\[(\d+)\]|^\['([^']*)'\]`)

// plainSteps splits a JSONPath made of member and index steps only
// ($.data.items[0]['x-id']).
func plainSteps(path string) ([]string, bool) {
	rest, ok := strings.CutPrefix(path, "$")
	if !ok {
		return nil, false
	}
	var out []string
	for rest != "" {
		m := plainStep.FindStringSubmatch(rest)
		if m == nil {
			return nil, false
		}
		out = append(out, m[1]+m[2]+m[3])
		rest = rest[len(m[0]):]
	}
	return out, true
}
