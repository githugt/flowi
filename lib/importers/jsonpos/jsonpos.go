// SPDX-License-Identifier: Apache-2.0

// Package jsonpos finds where values are in a JSON document, so importers
// of JSON formats (HAR, Postman) can report problems at a line and column.
package jsonpos

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Scan walks data once and returns the byte offset of every value whose
// path keep accepts. A path is object keys and array indexes from the
// root; the result is keyed by the path joined with "/".
func Scan(data []byte, keep func(path []string) bool) (map[string]int, error) {
	s := &scanner{data: data, dec: json.NewDecoder(bytes.NewReader(data)), keep: keep, out: map[string]int{}}
	if err := s.value(nil); err != nil {
		return nil, err
	}
	return s.out, nil
}

type scanner struct {
	data []byte
	dec  *json.Decoder
	keep func([]string) bool
	out  map[string]int
}

// start is where the next value begins: the decoder's offset sits before
// any whitespace, comma or colon.
func (s *scanner) start() int {
	off := int(s.dec.InputOffset())
	for off < len(s.data) && strings.IndexByte(" \t\r\n,:", s.data[off]) >= 0 {
		off++
	}
	return off
}

func (s *scanner) value(path []string) error {
	if s.keep(path) {
		s.out[strings.Join(path, "/")] = s.start()
	}
	tok, err := s.dec.Token()
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'):
		for s.dec.More() {
			key, err := s.dec.Token()
			if err != nil {
				return err
			}
			name, _ := key.(string) // object keys are always strings
			if err := s.value(append(path, name)); err != nil {
				return err
			}
		}
		_, err = s.dec.Token()
	case json.Delim('['):
		for i := 0; s.dec.More(); i++ {
			if err := s.value(append(path, strconv.Itoa(i))); err != nil {
				return err
			}
		}
		_, err = s.dec.Token()
	}
	return err
}

// ErrorOffset is where a decoding error is: a syntax error's offset, or
// size (the end) when the input stops early.
func ErrorOffset(err error, size int) (int, bool) {
	var se *json.SyntaxError
	switch {
	case errors.As(err, &se) && int(se.Offset) >= size:
		return size, true // the input stops early
	case errors.As(err, &se):
		return max(int(se.Offset)-1, 0), true
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return size, true
	}
	return 0, false
}
