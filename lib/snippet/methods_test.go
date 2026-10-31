// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func TestGenerate_Given_query_or_a_custom_method_Then_every_format_sends_that_exact_method(t *testing.T) {
	tests := []struct {
		format Format
		method string
		want   string
	}{
		{Curl, "QUERY", "curl -X QUERY "},
		{Curl, "PROPFIND", "curl -X PROPFIND "},
		{Wget, "QUERY", "--method=QUERY"},
		{PowerShell, "QUERY", "Invoke-RestMethod -CustomMethod 'QUERY' "},
		{PowerShell, "Purge", "-CustomMethod 'Purge'"},
		{PowerShell, "DELETE", "-Method Delete "},
		{Xh, "QUERY", "xh QUERY "},
	}
	for _, tt := range tests {
		t.Run(string(tt.format)+" "+tt.method, func(t *testing.T) {
			got, err := Generate(tt.format, engine.PreviewRequest{Method: tt.method, URL: "https://api.example.com/search"})

			if err != nil || !strings.Contains(got, tt.want) {
				t.Fatalf("err %v\n--- got\n%s\n--- want it to contain %q", err, got, tt.want)
			}
		})
	}
}
