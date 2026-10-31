// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/githugt/flowi/lib/internal/httpx"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The values of [MS-NLMP] 3.3.2 and 4.2.4: user "User" of "Domain" with password
// "Password", the server challenge 0123456789abcdef and the client challenge
// aaaaaaaaaaaaaaaa.
func TestNTLMv2_should_match_the_published_test_vectors(t *testing.T) {
	// Arrange
	serverChallenge := mustHex(t, "0123456789abcdef")
	clientChallenge := bytes.Repeat([]byte{0xaa}, 8)
	targetInfo := mustHex(t, "02000c0044006f006d00610069006e00 01000c00530065007200760065007200 00000000")

	// Act
	key := ntowfv2("Password", "User", "Domain")
	nt, _ := ntlmv2Response(key, serverChallenge, clientChallenge, targetInfo, 0)
	lm := lmv2Response(key, serverChallenge, clientChallenge)

	// Assert
	if got, want := hex.EncodeToString(key), "0c868a403bfd7a93a3001ef22ef02e3f"; got != want {
		t.Errorf("NTOWFv2 %s, want %s", got, want)
	}
	if got, want := hex.EncodeToString(nt[:16]), "68cd0ab851e51c96aabc927bebef6a1c"; got != want {
		t.Errorf("NTProofStr %s, want %s", got, want)
	}
	if got, want := hex.EncodeToString(lm), "86c35097ac9cec102554764a57cccc19aaaaaaaaaaaaaaaa"; got != want {
		t.Errorf("LMv2 response %s, want %s", got, want)
	}
}

func TestNTLM_should_send_a_negotiate_message_first(t *testing.T) {
	// Arrange
	s, err := New(auth("ntlm", "username", `CORP\ada`, "password", "pw"), fixedEnv(time.Unix(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	r := request("GET", "http://intranet/x")

	// Act
	applied, err := s.Apply(context.Background(), r)

	// Assert
	if err != nil || len(applied) != 1 {
		t.Fatalf("applied %v, err %v", applied, err)
	}
	h := r.Header.Get("Authorization")
	raw, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "NTLM "))
	if !strings.HasPrefix(h, "NTLM ") || decodeErr != nil || !bytes.HasPrefix(raw, []byte("NTLMSSP\x00")) || binary.LittleEndian.Uint32(raw[8:]) != 1 {
		t.Fatalf("header %q is not an NTLM negotiate message", h)
	}
	if applied[0].Secret {
		t.Error("a negotiate message carries no credential")
	}
}

// challengeMessage builds the server's NTLM challenge message.
func challengeMessage(t *testing.T, serverChallenge []byte, targetInfo []byte) string {
	t.Helper()
	const header = 48
	msg := make([]byte, header, header+len(targetInfo))
	copy(msg, "NTLMSSP\x00")
	binary.LittleEndian.PutUint32(msg[8:], 2)
	binary.LittleEndian.PutUint32(msg[20:], flagUnicode|flagNTLM|flagTargetInfo|flagExtendedSecurity)
	copy(msg[24:], serverChallenge)
	binary.LittleEndian.PutUint16(msg[40:], uint16(len(targetInfo)))
	binary.LittleEndian.PutUint16(msg[42:], uint16(len(targetInfo)))
	binary.LittleEndian.PutUint32(msg[44:], header)
	return base64.StdEncoding.EncodeToString(append(msg, targetInfo...))
}

func TestNTLM_should_answer_the_servers_challenge_with_an_NTLMv2_response(t *testing.T) {
	// Arrange
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	env := Env{Now: func() time.Time { return now }, Nonce: func(n int) string { return strings.Repeat("a", n) }}
	s, _ := New(auth("ntlm", "username", "ada", "password", "Password", "domain", "Domain"), env)
	targetInfo := mustHex(t, "02000c0044006f006d00610069006e00 01000c00530065007200760065007200 00000000")
	resp := &httpx.Response{Status: 401, Header: map[string][]string{"Www-Authenticate": {"NTLM " + challengeMessage(t, mustHex(t, "0123456789abcdef"), targetInfo)}}}
	r := request("GET", "http://intranet/x")
	_, _ = s.Apply(context.Background(), r) // the negotiate message went out

	// Act
	answered := s.(Challenger).Challenge(resp)
	applied, err := s.Apply(context.Background(), request2(r))

	// Assert
	if !answered || err != nil || len(applied) != 1 || !applied[0].Secret {
		t.Fatalf("answered %v, applied %v, err %v", answered, applied, err)
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(applied[0].Value, "NTLM "))
	if binary.LittleEndian.Uint32(raw[8:]) != 3 {
		t.Fatalf("the answer is message type %d, want 3", binary.LittleEndian.Uint32(raw[8:]))
	}
	user := payload(raw, 36)
	domain := payload(raw, 28)
	if decodeUTF16(user) != "ada" || decodeUTF16(domain) != "Domain" {
		t.Errorf("user %q domain %q", decodeUTF16(user), decodeUTF16(domain))
	}
	ntResponse := payload(raw, 20)
	if len(ntResponse) < 16+28 || bytes.Contains(raw, []byte("Password")) {
		t.Errorf("NT response of %d bytes; the password must not be in the message", len(ntResponse))
	}
}

// request2 is a request to send again after a challenge.
func request2(r *httpx.Request) *httpx.Request { return request(r.Method, r.URL) }

// payload reads the payload a security buffer points at (length at off, offset at off+4).
func payload(msg []byte, off int) []byte {
	n := int(binary.LittleEndian.Uint16(msg[off:]))
	at := int(binary.LittleEndian.Uint32(msg[off+4:]))
	return msg[at : at+n]
}

func decodeUTF16(b []byte) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(units))
}

func TestNTLM_Challenge_should_say_no_to_a_response_that_does_not_offer_NTLM(t *testing.T) {
	cases := map[string]*httpx.Response{
		"ok":                {Status: 200},
		"basic only":        {Status: 401, Header: map[string][]string{"Www-Authenticate": {"Basic realm=x"}}},
		"bare NTLM":         {Status: 401, Header: map[string][]string{"Www-Authenticate": {"NTLM"}}},
		"not a message":     {Status: 401, Header: map[string][]string{"Www-Authenticate": {"NTLM !!!"}}},
		"wrong message":     {Status: 401, Header: map[string][]string{"Www-Authenticate": {"NTLM " + base64.StdEncoding.EncodeToString([]byte("NTLMSSP\x00\x01\x00\x00\x00"))}}},
		"truncated message": {Status: 401, Header: map[string][]string{"Www-Authenticate": {"NTLM " + base64.StdEncoding.EncodeToString([]byte("NTLMSSP\x00\x02\x00\x00\x00abc"))}}},
	}
	for name, resp := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := New(auth("ntlm", "username", "ada", "password", "pw"), Env{})

			if s.(Challenger).Challenge(resp) {
				t.Error("an NTLM answer must wait for a well-formed challenge message")
			}
		})
	}
}

func TestNTLM_should_split_a_domain_from_the_user_name(t *testing.T) {
	cases := map[string][2]string{`CORP\ada`: {"CORP", "ada"}, "ada@corp.example": {"corp.example", "ada"}, "ada": {"", "ada"}}
	for in, want := range cases {
		domain, user := splitUser(in, "")
		if domain != want[0] || user != want[1] {
			t.Errorf("%q: got %q %q, want %q %q", in, domain, user, want[0], want[1])
		}
	}
	if domain, _ := splitUser("ada", "EXPLICIT"); domain != "EXPLICIT" {
		t.Error("the domain parameter names the domain when the user name does not")
	}
}
