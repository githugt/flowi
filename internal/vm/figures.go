// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"time"
)

const (
	kilobyte = 1000
	megabyte = 1000 * kilobyte
)

// TookText is how long a send took, in the unit that reads best: "820 µs", "142 ms", "1.24 s".
// A zero duration means nothing was measured and gives "".
func TookText(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Millisecond:
		return fmt.Sprintf("%d µs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

// SizeText is a body's size as "512 B", "1.2 KB" or "3.4 MB". A negative size means unknown and gives "";
// an empty body is "0 B" because that is a fact worth showing.
func SizeText(n int64) string {
	switch {
	case n < 0:
		return ""
	case n < kilobyte:
		return fmt.Sprintf("%d B", n)
	case n < megabyte:
		return fmt.Sprintf("%.1f KB", float64(n)/kilobyte)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/megabyte)
}
