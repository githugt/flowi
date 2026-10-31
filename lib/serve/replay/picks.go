// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Request headers that steer the replayer. They are removed from nothing:
// the replayer does not forward requests (except in record mode, which
// strips them).
const (
	HeaderStatus  = "X-Flowi-Status"
	HeaderExample = "X-Flowi-Example"
	HeaderRequest = "X-Flowi-Request"
	HeaderVersion = "X-Flowi-Version"
	HeaderLatency = "X-Flowi-Latency"
	HeaderMatch   = "X-Flowi-Match"
	HeaderSource  = "X-Flowi-Source"

	// maxLatency caps simulated delay so a typo cannot park a connection.
	maxLatency = 30 * time.Second
)

// picks are the choices a caller makes with X-Flowi-* headers.
type picks struct {
	request string
	version string
	example string
	status  int
	latency time.Duration
	// hasLatency tells an explicit 0s (no delay) from no header (use the
	// default latency).
	hasLatency bool
}

// readPicks reads the steering headers. A malformed one is an error the
// caller answers with 400, naming the header.
func readPicks(h http.Header) (picks, error) {
	p := picks{
		request: strings.TrimSpace(h.Get(HeaderRequest)),
		version: strings.TrimSpace(h.Get(HeaderVersion)),
		example: strings.TrimSpace(h.Get(HeaderExample)),
	}
	if v := strings.TrimSpace(h.Get(HeaderStatus)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 100 || n > 599 {
			return picks{}, fmt.Errorf("%s: %q is not a status code (100-599)", HeaderStatus, v)
		}
		p.status = n
	}
	if v := strings.TrimSpace(h.Get(HeaderLatency)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return picks{}, fmt.Errorf("%s: %q is not a duration such as 250ms or 2s", HeaderLatency, v)
		}
		p.latency, p.hasLatency = min(d, maxLatency), true
	}
	return p, nil
}

// narrow keeps the candidates the caller asked for with X-Flowi-Request
// and X-Flowi-Version.
func (p picks) narrow(cs []candidate) []candidate {
	var out []candidate
	for _, c := range cs {
		if p.request != "" && p.request != c.req.Path && p.request != string(c.req.ID) {
			continue
		}
		if p.version != "" && p.version != c.ver.Label {
			continue
		}
		out = append(out, c)
	}
	return out
}
