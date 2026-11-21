// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"reflect"
	"testing"
)

// Before: the editor command was split on whitespace, so a program path with spaces broke.
func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		windows bool
		want    []string
		wantErr bool
	}{
		{"plain", "vi", false, []string{"vi"}, false},
		{"flags", "code --wait  -n", false, []string{"code", "--wait", "-n"}, false},
		{"double quoted path", `"/opt/my editor/ed" -w`, false, []string{"/opt/my editor/ed", "-w"}, false},
		{"single quoted", `'a b' c`, false, []string{"a b", "c"}, false},
		{"escaped space", `/opt/my\ editor/ed`, false, []string{"/opt/my editor/ed"}, false},
		{"windows path", `"C:\Program Files\Vim\vim.exe" -u NONE`, true, []string{`C:\Program Files\Vim\vim.exe`, "-u", "NONE"}, false},
		{"windows backslash kept", `C:\Vim\vim.exe`, true, []string{`C:\Vim\vim.exe`}, false},
		{"empty quoted argument", `ed ""`, false, []string{"ed", ""}, false},
		{"empty", "  ", false, nil, false},
		{"unclosed quote", `"a b`, false, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandOS(tt.in, tt.windows)
			if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, %v; want %q (error %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
