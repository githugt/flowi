// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import flowi "github.com/githugt/flowi/lib"

// TokenStatus says where a saved OAuth token stands, in a few words for a row of the Authentications screen.
func TokenStatus(t flowi.AuthToken) string {
	switch {
	case !t.Expired:
		return "Signed in until " + t.Expires.Local().Format("15:04, 2 Jan")
	case t.HasRefresh:
		return "Expired: renews by itself on the next send"
	}
	return "Expired: the next send asks you to sign in again"
}
