// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Given_query_and_custom_methods_Then_the_server_sees_the_method_and_the_body(t *testing.T) {
	for _, method := range []string{"QUERY", "PROPFIND", "Purge"} {
		t.Run(method, func(t *testing.T) {
			// Arrange
			var gotMethod, gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				gotMethod, gotBody = r.Method, string(b)
			}))
			defer srv.Close()
			c := newClient(t, defaultOptions())

			// Act
			_, err := c.Do(context.Background(), Request{Method: method, URL: srv.URL, Header: http.Header{}, Body: BytesBody([]byte(`{"q":"x"}`), "application/json")})

			// Assert
			if err != nil || gotMethod != method || gotBody != `{"q":"x"}` {
				t.Fatalf("err %v, server saw %q with body %q", err, gotMethod, gotBody)
			}
		})
	}
}
