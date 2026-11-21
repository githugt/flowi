// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"fmt"
	"path/filepath"
	"strings"
)

// confine returns path when it is inside q.Root, or when q may leave it. Before: a file path joined onto the root
// as it was, so "../../home/me/.ssh/x" or an absolute path read any file the person could, from a cloned collection.
// A query with no Root (a try-out request that is not in a collection) is not confined.
func confine(q Query, path string) (string, error) {
	if q.Root == "" || q.AllowOutsideRoot {
		return path, nil
	}
	rel, err := filepath.Rel(filepath.Clean(q.Root), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("the path %q is outside the collection folder. To read it, turn on the setting datasets.allow_outside_collection (Settings > Requests); a collection cannot turn it on", q.Server)
	}
	return path, nil
}
