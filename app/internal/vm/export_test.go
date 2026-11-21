// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "time"

// SetWatchRetry shortens the wait before a failed watch is started again, so a test does not sleep.
func (ws *Workspace) SetWatchRetry(d time.Duration) { ws.watchRetry = d }

// WatchRetries is how often a failed watch is started again.
const WatchRetries = watchRetries
