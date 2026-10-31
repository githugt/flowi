// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Faults make the replayer misbehave on purpose, so a client's retries, timeouts and error handling can be
// tried against a server that is easy to break. Each call rolls once: a roll under DropRate drops the
// connection, the next ErrorRate of the range answers with ErrorStatus, and the rest is answered normally.
// A call can still ask for one outcome with the X-Flowi-Fault header.
type Faults struct {
	ErrorRate   float64       // share of calls answered with ErrorStatus, 0 to 1
	ErrorStatus int           // default 503
	DropRate    float64       // share of calls whose connection is closed with no answer, 0 to 1
	Jitter      time.Duration // up to this much extra delay on every answer, on top of the latency
}

// defaultFaultStatus is the status of an injected error when none is set: the one a busy server gives.
const defaultFaultStatus = http.StatusServiceUnavailable

// Active says whether any fault is configured.
func (f Faults) Active() bool { return f.ErrorRate > 0 || f.DropRate > 0 || f.Jitter > 0 }

// Validate refuses rates that mean nothing, so a typo fails at start instead of at the first call.
func (f Faults) Validate() error {
	switch {
	case f.ErrorRate < 0 || f.ErrorRate > 1:
		return fmt.Errorf("the error rate %v is not between 0 and 1", f.ErrorRate)
	case f.DropRate < 0 || f.DropRate > 1:
		return fmt.Errorf("the drop rate %v is not between 0 and 1", f.DropRate)
	case f.ErrorRate+f.DropRate > 1:
		return errors.New("the error rate and the drop rate together are more than 1")
	case f.ErrorStatus != 0 && (f.ErrorStatus < 400 || f.ErrorStatus > 599):
		return fmt.Errorf("the error status %d is not an error status (400-599)", f.ErrorStatus)
	case f.Jitter < 0:
		return errors.New("the jitter must not be negative")
	}
	return nil
}

// faultKind is what happens to one call.
type faultKind int

const (
	faultNone faultKind = iota
	faultError
	faultDrop
)

// fault is the outcome chosen for one call.
type fault struct {
	kind   faultKind
	status int
}

// HeaderFault lets a call ask for its own outcome: none, drop, error or error:<status>.
const HeaderFault = "X-Flowi-Fault"

// readFault reads X-Flowi-Fault. set is false when the header is absent.
func readFault(h http.Header) (f fault, set bool, err error) {
	v := strings.ToLower(strings.TrimSpace(h.Get(HeaderFault)))
	switch v {
	case "":
		return fault{}, false, nil
	case "none":
		return fault{}, true, nil
	case "drop":
		return fault{kind: faultDrop}, true, nil
	case "error":
		return fault{kind: faultError, status: defaultFaultStatus}, true, nil
	}
	if status, ok := strings.CutPrefix(v, "error:"); ok {
		n, convErr := strconv.Atoi(status)
		if convErr == nil && n >= 400 && n <= 599 {
			return fault{kind: faultError, status: n}, true, nil
		}
	}
	return fault{}, false, fmt.Errorf("%s: %q is not none, drop, error or error:<status 400-599>", HeaderFault, h.Get(HeaderFault))
}

// roll picks the outcome for one call from the configured rates.
func (f Faults) roll(r float64) fault {
	switch {
	case r < f.DropRate:
		return fault{kind: faultDrop}
	case r < f.DropRate+f.ErrorRate:
		status := f.ErrorStatus
		if status == 0 {
			status = defaultFaultStatus
		}
		return fault{kind: faultError, status: status}
	}
	return fault{}
}

// errorAnswer is the answer of an injected error.
func errorAnswer(r *http.Request, status int) answer {
	return problem(r, status, MatchFault, "injected fault: this error was made on purpose by the replayer")
}
