// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"net"
	"strings"
)

// SameHost reports whether two host:port addresses can be the same socket on this machine. Names are not resolved,
// but "localhost", loopback IPs (127.0.0.1, ::1, any 127.x.y.z) and an unspecified listen address (0.0.0.0, [::])
// count as one another when the ports are equal; anything else must match as written, ignoring case. An address
// without a port is compared by host alone.
func SameHost(a, b string) bool {
	ha, pa := splitHostPort(a)
	hb, pb := splitHostPort(b)
	if pa != pb {
		return false
	}
	if strings.EqualFold(ha, hb) {
		return true
	}
	return local(ha) && local(hb)
}

func splitHostPort(addr string) (host, port string) {
	if h, p, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(h, "[]"), p
	}
	return strings.Trim(addr, "[]"), ""
}

// local reports whether host names this machine's loopback, or every interface when it is a listen address.
func local(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}
