// SPDX-License-Identifier: Apache-2.0

package model

import "testing"

func TestGRPCTarget_should_pick_plaintext_or_tls_from_the_scheme_and_guess_for_a_bare_host(t *testing.T) {
	tests := []struct {
		server, hostport string
		tls              bool
	}{
		{"grpc://api:50051", "api:50051", false},
		{"grpcs://api.example.com:443", "api.example.com:443", true},
		{"localhost:50051", "localhost:50051", false},
		{"127.0.0.1:50051", "127.0.0.1:50051", false},
		{"[::1]:50051", "[::1]:50051", false},
		{"api.example.com:443", "api.example.com:443", true},
		{"api.example.com", "api.example.com", true},
		{"grpc://localhost:1/", "localhost:1", false},
	}
	for _, tt := range tests {
		hostport, useTLS := GRPCTarget(tt.server)
		if hostport != tt.hostport || useTLS != tt.tls {
			t.Errorf("GRPCTarget(%q) = %q, %v; want %q, %v", tt.server, hostport, useTLS, tt.hostport, tt.tls)
		}
	}
}

func TestSplitMethod_should_split_package_service_and_method(t *testing.T) {
	tests := []struct {
		in, service, method string
		ok                  bool
	}{
		{"acme.orders.v1.Orders/GetOrder", "acme.orders.v1.Orders", "GetOrder", true},
		{"Orders/Get", "Orders", "Get", true},
		{"/acme.Orders/Get", "acme.Orders", "Get", true}, // grpcurl's leading slash is allowed
		{"Orders", "", "", false},
		{"Orders/", "", "", false},
		{"/Get", "", "", false},
		{"a/b/c", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		service, method, ok := SplitMethod(tt.in)
		if service != tt.service || method != tt.method || ok != tt.ok {
			t.Errorf("SplitMethod(%q) = %q, %q, %v; want %q, %q, %v", tt.in, service, method, ok, tt.service, tt.method, tt.ok)
		}
	}
}

func TestGRPCTargetURL_should_show_the_server_and_method_with_the_scheme_the_call_will_use(t *testing.T) {
	tests := []struct{ server, method, want string }{
		{"localhost:50051", "a.B/C", "grpc://localhost:50051/a.B/C"},
		{"api.example.com:443", "a.B/C", "grpcs://api.example.com:443/a.B/C"},
		{"grpc://h:1", "a.B/C", "grpc://h:1/a.B/C"},
		{"{{env.api}}", "a.B/C", "{{env.api}}/a.B/C"},
		{"grpc://h:1", "", "grpc://h:1"},
	}
	for _, tt := range tests {
		if got := GRPCTargetURL(tt.server, tt.method); got != tt.want {
			t.Errorf("GRPCTargetURL(%q, %q) = %q, want %q", tt.server, tt.method, got, tt.want)
		}
	}
}
