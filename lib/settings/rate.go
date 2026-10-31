// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseRate reads a rate limit such as "20/s", "120/m" or "5/h" and
// returns the interval between two calls at that rate.
func ParseRate(s string) (time.Duration, error) {
	count, unit, ok := strings.Cut(strings.TrimSpace(s), "/")
	units := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour}
	per, known := units[unit]
	n, err := strconv.Atoi(count)
	if !ok || !known || err != nil || n < 1 {
		return 0, fmt.Errorf("%q is not a rate (use <count>/s, /m or /h, e.g. 20/s)", s)
	}
	return per / time.Duration(n), nil
}
