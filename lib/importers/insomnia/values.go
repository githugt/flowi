// SPDX-License-Identifier: Apache-2.0

package insomnia

import "github.com/githugt/flowi-lib/internal/loose"

func asMap(v any) map[string]any           { return loose.Map(v) }
func list(v any) []map[string]any          { return loose.List(v) }
func str(v any) string                     { return loose.Str(v) }
func flag(v any) bool                      { return loose.Flag(v) }
func sortedKeys(m map[string]any) []string { return loose.SortedKeys(m) }
