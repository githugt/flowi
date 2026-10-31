// SPDX-License-Identifier: Apache-2.0

package sites

import (
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/version"
)

// ResolvedVersion is the version a node runs without a run preference.
func ResolvedVersion(r model.Request, pinned, prefer string) (model.RequestVersion, bool) {
	q := version.Query{Pinned: ParseLabel(pinned), FlowPrefer: ParseLabel(prefer), Default: ParseLabel(r.Default)}
	byLabel := map[string]model.RequestVersion{}
	for _, v := range r.Versions {
		if l, err := version.Parse(v.Label); err == nil {
			q.Available = append(q.Available, l)
			byLabel[l.String()] = v
		}
	}
	l, _, err := version.Resolve(q)
	if err != nil {
		return model.RequestVersion{}, false
	}
	v, ok := byLabel[l.String()]
	return v, ok
}

// ParseLabel parses a version label; empty or invalid labels give nil.
func ParseLabel(s string) *version.Label {
	if s == "" {
		return nil
	}
	l, err := version.Parse(s)
	if err != nil {
		return nil
	}
	return &l
}
