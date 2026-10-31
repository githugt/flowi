// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"time"

	flowi "github.com/githugt/flowi-lib"
)

// generateTimeout bounds GenerateRequests: it writes a file per method, so
// anything near this long means the actor is stuck, not that it is busy.
const generateTimeout = 30 * time.Second

// GeneratedView says what Generate collection made.
type GeneratedView struct {
	Created int      `json:"created"`
	Skipped []string `json:"skipped"` // one line per method: why no request was made
}

var (
	errNoMethods       = errors.New("load the methods first")
	errGenerateTimeout = errors.New("making the requests took too long")
)

// GenerateRequests makes a gRPC request for each loaded method, in the
// request's collection under grpc/, with its server and schema files.
func (s *RequestService) GenerateRequests(node string) (GeneratedView, error) {
	type result struct {
		g   flowi.GeneratedGRPC
		err error
	}
	// Buffered: the callbacks run on the actor after with returns, and must
	// not block if the wait below has given up.
	done := make(chan result, 1)
	started := false
	s.with(node, func(o *openRequest) {
		if !o.req.IsGRPC() || len(o.req.Methods()) == 0 {
			return
		}
		started = true
		s.app.send.GenerateRequests(o.req,
			func(g flowi.GeneratedGRPC) { done <- result{g: g} },
			func(err error) { done <- result{err: err} })
	})
	if !started {
		return GeneratedView{}, errNoMethods
	}
	select {
	case r := <-done:
		if r.err != nil {
			return GeneratedView{}, fmt.Errorf("making the gRPC requests: %w", r.err)
		}
		return GeneratedView{Created: len(r.g.Created), Skipped: append([]string{}, r.g.Skipped...)}, nil
	case <-time.After(generateTimeout):
		return GeneratedView{}, errGenerateTimeout
	}
}
