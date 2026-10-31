// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // Hawk names sha1 as an algorithm a server may use
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/internal/httpx"
)

// hawk signs a request with a Hawk header (the HTTP authentication scheme of
// Mozilla's services): an HMAC over the method, resource, host, port, time
// and a nonce, and over the payload when there is a body.
type hawk struct {
	id, key, algorithm, ext, app, dlg string
	newHash                           func() hash.Hash
	env                               Env
}

func newHawk(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("id", "key"); err != nil {
		return nil, err
	}
	h := &hawk{id: p.values["id"], key: p.values["key"], algorithm: strings.ToLower(p.get("algorithm", "sha256")),
		ext: p.values["ext"], app: p.values["app"], dlg: p.values["dlg"], env: env}
	switch h.algorithm {
	case "sha256":
		h.newHash = sha256.New
	case "sha1":
		h.newHash = sha1.New
	default:
		return nil, fmt.Errorf("hawk auth: algorithm must be sha256 or sha1, got %q", h.algorithm)
	}
	return h, nil
}

func (h *hawk) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("hawk auth: %w", err)
	}
	body, err := bodyBytes(r)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt(h.env.now().Unix(), 10)
	nonce := h.env.nonce(6)
	payload := ""
	if len(body) > 0 {
		payload = h.payloadHash(r.Header.Get("Content-Type"), body)
	}
	mac := h.mac(r.Method, u, ts, nonce, payload)
	value := fmt.Sprintf(`Hawk id=%q, ts=%q, nonce=%q`, h.id, ts, nonce)
	if payload != "" {
		value += fmt.Sprintf(`, hash=%q`, payload)
	}
	if h.ext != "" {
		value += fmt.Sprintf(`, ext=%q`, h.ext)
	}
	value += fmt.Sprintf(`, mac=%q`, mac)
	if h.app != "" {
		value += fmt.Sprintf(`, app=%q`, h.app)
		if h.dlg != "" {
			value += fmt.Sprintf(`, dlg=%q`, h.dlg)
		}
	}
	r.Header.Set("Authorization", value)
	return []Applied{{Where: "header", Name: "Authorization", Value: value, Secret: true}}, nil
}

// mac is the request MAC over Hawk's normalized request string.
func (h *hawk) mac(method string, u *url.URL, ts, nonce, payloadHash string) string {
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443"}[strings.ToLower(u.Scheme)]
		if port == "" {
			port = "80"
		}
	}
	lines := []string{"hawk.1.header", ts, nonce, strings.ToUpper(method), u.RequestURI(), strings.ToLower(u.Hostname()), port, payloadHash, h.ext}
	if h.app != "" {
		lines = append(lines, h.app, h.dlg)
	}
	m := hmac.New(h.newHash, []byte(h.key))
	m.Write([]byte(strings.Join(lines, "\n") + "\n"))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

// payloadHash is the Hawk hash of a body: of its media type (no parameters,
// lower case) and its bytes.
func (h *hawk) payloadHash(contentType string, body []byte) string {
	media, _, _ := strings.Cut(contentType, ";")
	d := h.newHash()
	d.Write([]byte("hawk.1.payload\n" + strings.ToLower(strings.TrimSpace(media)) + "\n"))
	d.Write(body)
	d.Write([]byte("\n"))
	return base64.StdEncoding.EncodeToString(d.Sum(nil))
}
