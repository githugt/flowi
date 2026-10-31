// SPDX-License-Identifier: Apache-2.0

package all_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport/all"
)

func TestNew_should_speak_every_protocol_the_model_knows(t *testing.T) {
	reg := all.New()

	for _, p := range model.AsyncProtocols() {
		if _, ok := reg.Lookup(p); !ok {
			t.Errorf("no transport for %s", p)
		}
	}
}
