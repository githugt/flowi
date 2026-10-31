// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Limits of FetchImportSource: a spec is text, and the server is the user's
// own choice, but a wrong URL must not hang the import or fill memory.
const (
	fetchTimeout = 30 * time.Second
	fetchMaxSize = 16 << 20
)

// IsImportURL reports whether ref is an http or https URL to fetch rather than
// a file to read.
func IsImportURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// FetchImportSource downloads the document at rawURL (an OpenAPI or AsyncAPI
// description served by an API, say) and returns it with a name for reports,
// the last segment of the URL path ("openapi.json"). It sends no credentials
// and follows redirects; a status other than 200 is an error.
func FetchImportSource(ctx context.Context, rawURL string) (data []byte, name string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil || !IsImportURL(rawURL) || u.Host == "" {
		return nil, "", fmt.Errorf("%q is not an http or https URL", rawURL)
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("fetching %s: %w", u.Redacted(), err)
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*;q=0.5")
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // the URL is the one the user asked to import from
	if err != nil {
		return nil, "", fmt.Errorf("fetching %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close() //nolint:errcheck // a body already read
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetching %s: the server answered %s", u.Redacted(), resp.Status)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, fetchMaxSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", u.Redacted(), err)
	}
	if len(data) > fetchMaxSize {
		return nil, "", fmt.Errorf("%s is larger than %d MiB", u.Redacted(), fetchMaxSize>>20)
	}
	if base := path.Base(u.Path); base != "." && base != "/" {
		name = base
	}
	return data, name, nil
}
