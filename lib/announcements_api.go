// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"path/filepath"

	"github.com/githugt/flowi-lib/announce"
)

// announcementsCacheName is the announcements cache inside the config
// directory.
const announcementsCacheName = "announcements.cache.json"

// Announcements returns the project's signed announcements per the user's
// settings. It returns announce.ErrTurnedOff or announce.ErrNotConfigured
// when there is nothing to fetch; any other error comes with the cached
// items, if there are some (see announce.Feed.Fetch). refresh ignores a
// recent cache.
func (w *Workspace) Announcements(ctx context.Context, refresh bool) (announce.Result, error) {
	cache := filepath.Join(w.opts.Paths.ConfigDir, announcementsCacheName)
	return announce.Load(ctx, w.Settings(Scope{}), cache, refresh)
}
