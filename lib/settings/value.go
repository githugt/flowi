// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var errWrongType = errors.New("wrong type")

// ParseValue converts a raw value from YAML (string, bool, int64, []any)
// or from an environment variable (always string) into the key's type:
// string, bool, int64, time.Duration or []string.
func ParseValue(k Key, raw any) (any, error) {
	parsers := map[Kind]func(Key, any) (any, error){
		KindString:     parseString,
		KindBool:       parseBool,
		KindInt:        parseInt,
		KindDuration:   parseDuration,
		KindEnum:       parseEnum,
		KindStringList: parseStringList,
	}
	parse, ok := parsers[k.Kind]
	if !ok {
		return nil, fmt.Errorf("setting %s has unknown kind %d", k.Name, k.Kind)
	}
	return parse(k, raw)
}

func parseString(_ Key, raw any) (any, error) {
	s, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("%w: want a string, got %T", errWrongType, raw)
	}
	return s, nil
}

func parseBool(_ Key, raw any) (any, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		// Only true/false: YAML-1.1 style yes/no/on/off are a common source of surprises.
		if v == "true" || v == "false" {
			return v == "true", nil
		}
	}
	return nil, fmt.Errorf("want true or false, got %v", raw)
}

func parseInt(k Key, raw any) (any, error) {
	var n int64
	switch v := raw.(type) {
	case int64:
		n = v
	case int:
		n = int64(v)
	case uint64:
		if v > uint64(1<<63-1) {
			return nil, fmt.Errorf("%d is too large", v)
		}
		n = int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("want an integer, got %q", v)
		}
		n = parsed
	default:
		return nil, fmt.Errorf("%w: want an integer, got %T", errWrongType, raw)
	}
	if n < k.Min || (k.Max != 0 && n > k.Max) {
		return nil, fmt.Errorf("%d is outside %s", n, boundsText(k))
	}
	return n, nil
}

func boundsText(k Key) string {
	if k.Max == 0 {
		return fmt.Sprintf("the allowed range (at least %d)", k.Min)
	}
	return fmt.Sprintf("the allowed range %d–%d", k.Min, k.Max)
}

func parseDuration(_ Key, raw any) (any, error) {
	s, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("%w: want a duration such as 30s, got %T", errWrongType, raw)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, fmt.Errorf("want a duration such as 30s, got %q", s)
	}
	return d, nil
}

func parseEnum(k Key, raw any) (any, error) {
	s, ok := raw.(string)
	if !ok || !slices.Contains(k.Enum, s) {
		return nil, fmt.Errorf("want one of %s, got %v", strings.Join(k.Enum, ", "), raw)
	}
	return s, nil
}

// parseStringList accepts a YAML sequence of strings, or a comma-separated
// string from an environment variable. When the key has an Enum, every
// element must be one of it.
func parseStringList(k Key, raw any) (any, error) {
	list, err := stringList(raw)
	if err != nil || len(k.Enum) == 0 {
		return list, err
	}
	for _, item := range list.([]string) {
		if !slices.Contains(k.Enum, item) {
			return nil, fmt.Errorf("list items must be one of %s, got %q", strings.Join(k.Enum, ", "), item)
		}
	}
	return list, nil
}

func stringList(raw any) (any, error) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return []string{}, nil
		}
		parts := strings.Split(v, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p) // "5xx, 429" as people type it
		}
		return parts, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%w: list items must be strings, got %T", errWrongType, item)
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return slices.Clone(v), nil
	}
	return nil, fmt.Errorf("%w: want a list of strings, got %T", errWrongType, raw)
}
