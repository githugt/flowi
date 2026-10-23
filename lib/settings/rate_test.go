// SPDX-License-Identifier: Apache-2.0

package settings_test

import (
	"testing"
	"time"

	"github.com/githugt/flowi-lib/settings"
)

func TestParseRate(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "20/s", want: 50 * time.Millisecond},
		{in: "1/s", want: time.Second},
		{in: "120/m", want: 500 * time.Millisecond},
		{in: "5/h", want: 12 * time.Minute},
		{in: " 4/s ", want: 250 * time.Millisecond},
		{in: "0/s", wantErr: true},
		{in: "-1/s", wantErr: true},
		{in: "x", wantErr: true},
		{in: "5/d", wantErr: true},
		{in: "5", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := settings.ParseRate(tt.in)

			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("ParseRate(%q) = %v, %v", tt.in, got, err)
			}
		})
	}
}
