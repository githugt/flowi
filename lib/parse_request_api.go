// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"github.com/githugt/flowi/lib/diag"
	yamlcodec "github.com/githugt/flowi/lib/internal/codec/yaml"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// ParseRequest reads text in the format of a request file (`*.req.yaml`) into a
// request, without a workspace and without writing anything. name labels the
// text in diagnostics. A window uses it to fill a request from a request file
// that was pasted into it.
func ParseRequest(data []byte, name string) (model.Request, diag.List) {
	return yamlcodec.New(settings.Schema()).DecodeRequest(name, "", data)
}
