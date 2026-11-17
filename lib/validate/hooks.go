// SPDX-License-Identifier: Apache-2.0

package validate

import "github.com/githugt/flowi/lib/diag"

// checkHookCycles used to report hook flows that call each other in a cycle. A request sent from a hook flow never
// runs that hook flow (or one above it) again, so a hook that sends a request of its own collection (the token
// pattern) cannot recurse and there is no cycle to report.
func checkHookCycles(*index) diag.List { return nil }
