// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // NTLMv2 is defined on HMAC-MD5; a server that asks for NTLM gets it
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/crypto/md4" //nolint:gosec,staticcheck // the NT hash is MD4 by definition

	"github.com/githugt/flowi/lib/internal/httpx"
)

// NTLM flags (MS-NLMP 2.2.2.5) the client asks for and checks.
const (
	flagUnicode          = 0x00000001
	flagRequestTarget    = 0x00000004
	flagNTLM             = 0x00000200
	flagAlwaysSign       = 0x00008000
	flagExtendedSecurity = 0x00080000
	flagTargetInfo       = 0x00800000
	flag128              = 0x20000000
	flag56               = 0x80000000

	ntlmSignature  = "NTLMSSP\x00"
	ntlmHeaderSize = 64 // type 3 header without version and MIC
	ntlmMaxMessage = 8 << 10
)

// ntlm authenticates with NTLMv2 (MS-NLMP), the challenge-response scheme of
// Windows servers behind IIS and of many corporate proxies: the request goes
// out with a negotiate message, the 401 carries the server's challenge, and
// the request goes again with the answer, on the same connection. No signing
// or sealing is negotiated, and Kerberos (Negotiate) is not supported: a
// server that offers NTLM among its schemes works, one that offers only
// Negotiate with Kerberos does not.
type ntlm struct {
	domain, user, password, workstation string
	env                                 Env
	challenge                           *ntlmChallenge
}

type ntlmChallenge struct {
	serverChallenge []byte
	targetInfo      []byte
	flags           uint32
}

func newNTLM(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("username", "password"); err != nil {
		return nil, err
	}
	domain, user := splitUser(p.values["username"], p.values["domain"])
	return &ntlm{domain: domain, user: user, password: p.values["password"], workstation: p.values["workstation"], env: env}, nil
}

// splitUser separates a domain from the user name: "DOMAIN\user" and
// "user@domain" carry one; the domain parameter is used when they do not.
func splitUser(name, domain string) (string, string) {
	if d, u, ok := strings.Cut(name, `\`); ok {
		return d, u
	}
	if u, d, ok := strings.Cut(name, "@"); ok {
		return d, u
	}
	return domain, name
}

// Apply sends the negotiate message, or, once the server's challenge is
// known, the answer to it.
func (n *ntlm) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	if n.challenge == nil {
		value := "NTLM " + base64.StdEncoding.EncodeToString(n.negotiateMessage())
		r.Header.Set("Authorization", value)
		return []Applied{{Where: "header", Name: "Authorization", Value: value}}, nil
	}
	msg, err := n.authenticateMessage()
	if err != nil {
		return nil, err
	}
	value := "NTLM " + base64.StdEncoding.EncodeToString(msg)
	r.Header.Set("Authorization", value)
	return []Applied{{Where: "header", Name: "Authorization", Value: value, Secret: true}}, nil
}

// Challenge reads a 401's NTLM challenge message.
func (n *ntlm) Challenge(resp *httpx.Response) bool {
	if resp.Status != 401 || n.challenge != nil {
		return false
	}
	for _, line := range resp.Header.Values("Www-Authenticate") {
		scheme, token, _ := strings.Cut(strings.TrimSpace(line), " ")
		if !strings.EqualFold(scheme, "NTLM") || strings.TrimSpace(token) == "" {
			continue
		}
		if c, err := parseChallenge(strings.TrimSpace(token)); err == nil {
			n.challenge = &c
			return true
		}
	}
	return false
}

// negotiateMessage is the type 1 message: the flags asked for and no names.
func (n *ntlm) negotiateMessage() []byte {
	const size = 32
	msg := make([]byte, size)
	copy(msg, ntlmSignature)
	binary.LittleEndian.PutUint32(msg[8:], 1)
	binary.LittleEndian.PutUint32(msg[12:], flagUnicode|flagRequestTarget|flagNTLM|flagAlwaysSign|flagExtendedSecurity|flagTargetInfo|flag128|flag56)
	binary.LittleEndian.PutUint32(msg[20:], size) // an empty domain buffer, then an empty workstation buffer, both at the end
	binary.LittleEndian.PutUint32(msg[28:], size)
	return msg
}

// parseChallenge reads the type 2 message the server answered with.
func parseChallenge(token string) (ntlmChallenge, error) {
	msg, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ntlmChallenge{}, fmt.Errorf("the NTLM challenge is not base64: %w", err)
	}
	if len(msg) < 32 || len(msg) > ntlmMaxMessage || string(msg[:8]) != ntlmSignature || binary.LittleEndian.Uint32(msg[8:]) != 2 {
		return ntlmChallenge{}, errors.New("the NTLM challenge is not a type 2 message")
	}
	c := ntlmChallenge{serverChallenge: msg[24:32], flags: binary.LittleEndian.Uint32(msg[20:])}
	if c.flags&flagTargetInfo != 0 && len(msg) >= 48 {
		length, offset := int(binary.LittleEndian.Uint16(msg[40:])), int(binary.LittleEndian.Uint32(msg[44:]))
		if offset < 0 || offset+length > len(msg) {
			return ntlmChallenge{}, errors.New("the NTLM challenge's target information is outside the message")
		}
		c.targetInfo = msg[offset : offset+length]
	}
	return c, nil
}

// authenticateMessage is the type 3 message: who, and the proof of knowing
// the password for this challenge.
func (n *ntlm) authenticateMessage() ([]byte, error) {
	client, err := n.clientChallenge()
	if err != nil {
		return nil, err
	}
	key := ntowfv2(n.password, n.user, n.domain)
	nt, _ := ntlmv2Response(key, n.challenge.serverChallenge, client, n.challenge.targetInfo, filetime(n.env.now()))
	lm := lmv2Response(key, n.challenge.serverChallenge, client)
	fields := [][]byte{lm, nt, utf16le(n.domain), utf16le(n.user), utf16le(n.workstation), nil}
	msg := make([]byte, ntlmHeaderSize)
	copy(msg, ntlmSignature)
	binary.LittleEndian.PutUint32(msg[8:], 3)
	offset := ntlmHeaderSize
	for i, f := range fields {
		if len(f) > math.MaxUint16 {
			return nil, errors.New("ntlm auth: a name is too long for the message")
		}
		binary.LittleEndian.PutUint16(msg[12+8*i:], uint16(len(f))) //nolint:gosec // bounded just above
		binary.LittleEndian.PutUint16(msg[14+8*i:], uint16(len(f))) //nolint:gosec // bounded just above
		binary.LittleEndian.PutUint32(msg[16+8*i:], uint32(offset)) //nolint:gosec // the message is bounded by ntlmMaxMessage
		msg = append(msg, f...)
		offset += len(f)
	}
	binary.LittleEndian.PutUint32(msg[60:], flagUnicode|flagNTLM|flagExtendedSecurity|flagTargetInfo|flag128|flag56)
	return msg, nil
}

// clientChallenge is eight random bytes; tests fix it through Env.Nonce.
func (n *ntlm) clientChallenge() ([]byte, error) {
	if n.env.Nonce != nil {
		return []byte(n.env.Nonce(8)), nil
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("ntlm auth: no randomness: %w", err)
	}
	return b, nil
}

// ntowfv2 is the NTLMv2 key: HMAC-MD5 of the NT hash of the password over
// the upper-cased user name and the domain.
func ntowfv2(password, user, domain string) []byte {
	h := md4.New() //nolint:gosec // see the import
	h.Write(utf16le(password))
	return hmacMD5(h.Sum(nil), utf16le(strings.ToUpper(user)+domain))
}

// ntlmv2Response is NTProofStr followed by the blob it proves, and the blob.
func ntlmv2Response(key, serverChallenge, clientChallenge, targetInfo []byte, timestamp uint64) (response, blob []byte) {
	blob = []byte{1, 1, 0, 0, 0, 0, 0, 0}
	blob = binary.LittleEndian.AppendUint64(blob, timestamp)
	blob = append(blob, clientChallenge...)
	blob = append(blob, 0, 0, 0, 0)
	blob = append(blob, targetInfo...)
	blob = append(blob, 0, 0, 0, 0)
	proof := hmacMD5(key, append(append([]byte{}, serverChallenge...), blob...))
	return append(proof, blob...), blob
}

// lmv2Response is the LMv2 answer: a proof over both challenges, then the client challenge.
func lmv2Response(key, serverChallenge, clientChallenge []byte) []byte {
	proof := hmacMD5(key, append(append([]byte{}, serverChallenge...), clientChallenge...))
	return append(proof, clientChallenge...)
}

func hmacMD5(key, data []byte) []byte {
	h := hmac.New(md5.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func utf16le(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[2*i:], u)
	}
	return out
}

// filetime is t as a Windows FILETIME: 100 ns ticks since 1601.
func filetime(t time.Time) uint64 {
	const epochDifference = 11644473600                                             // seconds from 1601 to 1970
	return uint64(t.Unix()+epochDifference)*10_000_000 + uint64(t.Nanosecond()/100) //nolint:gosec // dates of this century
}
