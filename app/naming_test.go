// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

// Before: the changelog, which the Welcome tab shows as "What's new", called the product "Flow-I".
func TestChangelog_should_spell_the_product_name_one_way(t *testing.T) {
	if strings.Contains(changelog, "Flow-I") {
		t.Fatal(`the changelog says "Flow-I"; the product is called "Flowi"`)
	}
}
