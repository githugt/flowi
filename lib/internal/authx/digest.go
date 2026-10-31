// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/md5" //nolint:gosec // HTTP Digest's default algorithm (RFC 7616 keeps it); a server that asks for it gets it
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// digest answers a Digest challenge (RFC 7616; qop auth or none, MD5, SHA-256
// and SHA-512/256 as the server asks). The request first goes out without
// credentials, the 401 supplies the challenge, and it goes again with the
// answer, as every HTTP client does.
type digest struct {
	user, password string
	env            Env
	challenge      *digestChallenge
	nc             int
}

type digestChallenge struct {
	realm, nonce, opaque, algorithm, qop string
	userhash                             bool
}

func newDigest(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("username", "password"); err != nil {
		return nil, err
	}
	return &digest{user: p.values["username"], password: p.values["password"], env: env}, nil
}

// Apply adds the answer once a challenge is known; before, the request goes
// out as it is.
func (d *digest) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	if d.challenge == nil {
		return nil, nil
	}
	h, err := d.header(r)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", h)
	return []Applied{{Where: "header", Name: "Authorization", Value: h, Secret: true}}, nil
}

// Challenge reads a 401's WWW-Authenticate for a Digest challenge.
func (d *digest) Challenge(resp *httpx.Response) bool {
	if resp.Status != 401 || d.challenge != nil {
		return false
	}
	for _, line := range resp.Header.Values("Www-Authenticate") {
		if c, ok := parseDigest(line); ok {
			d.challenge = &c
			return true
		}
	}
	return false
}

func parseDigest(line string) (digestChallenge, bool) {
	scheme, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	if !strings.EqualFold(scheme, "digest") {
		return digestChallenge{}, false
	}
	c := digestChallenge{algorithm: "MD5"}
	for name, value := range authParams(rest) {
		switch strings.ToLower(name) {
		case "realm":
			c.realm = value
		case "nonce":
			c.nonce = value
		case "opaque":
			c.opaque = value
		case "algorithm":
			c.algorithm = value
		case "qop":
			c.qop = value
		case "userhash":
			c.userhash = strings.EqualFold(value, "true")
		}
	}
	return c, c.nonce != ""
}

// authParams reads name=value and name="value" pairs separated by commas.
func authParams(s string) map[string]string {
	out := map[string]string{}
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		name, rest, found := strings.Cut(s, "=")
		if !found {
			break
		}
		name = strings.TrimSpace(name)
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				value, s = rest[1:], ""
			} else {
				value, s = rest[1:1+end], rest[2+end:]
			}
		} else {
			value, s, _ = strings.Cut(rest, ",")
			value = strings.TrimSpace(value)
		}
		out[name] = value
	}
	return out
}

func (d *digest) hasher() (func() hash.Hash, string, error) {
	switch strings.ToUpper(strings.TrimSuffix(d.challenge.algorithm, "-sess")) {
	case "MD5", "":
		return md5.New, "MD5", nil
	case "SHA-256":
		return sha256.New, "SHA-256", nil
	case "SHA-512-256":
		return sha512.New512_256, "SHA-512-256", nil
	}
	return nil, "", fmt.Errorf("digest auth: the server asks for the algorithm %q, which is not supported", d.challenge.algorithm)
}

func (d *digest) header(r *httpx.Request) (string, error) {
	newHash, name, err := d.hasher()
	if err != nil {
		return "", err
	}
	sum := func(parts ...string) string {
		h := newHash()
		h.Write([]byte(strings.Join(parts, ":")))
		return hex.EncodeToString(h.Sum(nil))
	}
	u, err := url.Parse(r.URL)
	if err != nil {
		return "", fmt.Errorf("digest auth: %w", err)
	}
	uri := u.RequestURI()
	c := *d.challenge
	d.nc++
	nc, cnonce := fmt.Sprintf("%08x", d.nc), d.env.nonce(16)
	ha1 := sum(d.user, c.realm, d.password)
	if strings.HasSuffix(strings.ToLower(c.algorithm), "-sess") {
		ha1 = sum(ha1, c.nonce, cnonce)
	}
	ha2 := sum(r.Method, uri)
	qop := pickQOP(c.qop)
	response := sum(ha1, c.nonce, ha2)
	if qop != "" {
		response = sum(ha1, c.nonce, nc, cnonce, qop, ha2)
	}
	user := d.user
	if c.userhash {
		user = sum(d.user, c.realm)
	}
	fields := []string{
		field("username", user), field("realm", c.realm), field("nonce", c.nonce), field("uri", uri),
		"algorithm=" + name, field("response", response),
	}
	if qop != "" {
		fields = append(fields, "qop="+qop, "nc="+nc, field("cnonce", cnonce))
	}
	if c.opaque != "" {
		fields = append(fields, field("opaque", c.opaque))
	}
	if c.userhash {
		fields = append(fields, "userhash=true")
	}
	return "Digest " + strings.Join(fields, ", "), nil
}

func field(name, value string) string { return name + "=" + strconv.Quote(value) }

// pickQOP chooses auth when the server offers it ("auth-int" needs the body
// hash and is not used), and none when it names no qop (RFC 2069).
func pickQOP(offered string) string {
	for _, q := range strings.Split(offered, ",") {
		if strings.TrimSpace(q) == "auth" {
			return "auth"
		}
	}
	return ""
}
