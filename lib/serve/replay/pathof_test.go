// SPDX-License-Identifier: Apache-2.0

package replay

import "testing"

func TestPathOf_should_keep_only_the_path_the_replayer_answers_on(t *testing.T) {
	cases := map[string]string{
		"{{env.base_url}}/v1/users/{{id}}?x=1": "/v1/users/{{id}}",
		"https://api.example.com/v1/login#top": "/v1/login",
		"https://api.example.com":              "",
		"/plain/path":                          "/plain/path",
	}
	for in, want := range cases {
		if got := PathOf(in); got != want {
			t.Errorf("PathOf(%q) = %q, want %q", in, got, want)
		}
	}
}
