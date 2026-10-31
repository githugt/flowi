// SPDX-License-Identifier: Apache-2.0

// Package sharedtest is a small in-process stand-in for a Valkey or Redis
// server, for tests of code that talks to one. It speaks just enough RESP:
// AUTH, PING, SET (NX, PX), GET and DEL, with expiry on a clock the test
// moves by hand.
package sharedtest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Server is a fake Valkey listening on loopback.
type Server struct {
	// Addr is host:port to connect to.
	Addr string

	ln       net.Listener
	password string

	mu     sync.Mutex // guards everything below
	data   map[string]entry
	now    time.Time
	conns  map[net.Conn]struct{}
	opened int
	cmds   []string
}

type entry struct {
	value   []byte
	expires time.Time // zero: never
}

// Start launches a server that asks for password when it is not empty. It
// stops when the test ends.
func Start(t testing.TB, password string) *Server {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Addr: ln.Addr().String(), ln: ln, password: password, data: map[string]entry{}, now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), conns: map[net.Conn]struct{}{}}
	go s.accept()
	t.Cleanup(s.Close)
	return s
}

// Close stops listening and drops every connection.
func (s *Server) Close() {
	_ = s.ln.Close() // already closed when a test stopped the server early
	s.DropConnections()
}

// DropConnections closes every client connection but keeps listening, as a
// restart of a proxy or an idle timeout would.
func (s *Server) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		_ = c.Close() // the client sees EOF either way
	}
	s.conns = map[net.Conn]struct{}{}
}

// Advance moves the server's clock, expiring keys.
func (s *Server) Advance(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = s.now.Add(d)
}

// Get returns a key's value as the server holds it.
func (s *Server) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live(key)
	return string(e.value), ok
}

// Keys lists the live keys.
func (s *Server) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.data {
		if _, ok := s.live(k); ok {
			out = append(out, k)
		}
	}
	return out
}

// Connections is how many client connections have been accepted so far.
func (s *Server) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opened
}

// Commands lists the command names received, in order.
func (s *Server) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.cmds...)
}

func (s *Server) live(key string) (entry, bool) {
	e, ok := s.data[key]
	if !ok || (!e.expires.IsZero() && !e.expires.After(s.now)) {
		delete(s.data, key)
		return entry{}, false
	}
	return e, true
}

func (s *Server) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns[c] = struct{}{}
		s.opened++
		s.mu.Unlock()
		go s.serve(c)
	}
}

func (s *Server) serve(c net.Conn) {
	defer func() { _ = c.Close() }() // the peer may have closed first
	r := bufio.NewReader(c)
	authed := s.password == ""
	for {
		args, err := readCommand(r)
		if err != nil {
			return
		}
		reply := s.run(args, &authed)
		if _, err := io.WriteString(c, reply); err != nil {
			return
		}
	}
}

func (s *Server) run(args []string, authed *bool) string {
	name := strings.ToUpper(args[0])
	s.mu.Lock()
	s.cmds = append(s.cmds, name)
	s.mu.Unlock()
	if name == "AUTH" {
		if len(args) == 2 && args[1] == s.password {
			*authed = true
			return "+OK\r\n"
		}
		return "-WRONGPASS invalid username-password pair or user is disabled.\r\n"
	}
	if !*authed {
		return "-NOAUTH Authentication required.\r\n"
	}
	switch name {
	case "PING":
		return "+PONG\r\n"
	case "GET":
		return s.get(args)
	case "SET":
		return s.set(args)
	case "DEL":
		return s.del(args)
	}
	return fmt.Sprintf("-ERR unknown command '%s'\r\n", args[0])
}

func (s *Server) get(args []string) string {
	if len(args) != 2 {
		return "-ERR wrong number of arguments for 'get' command\r\n"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live(args[1])
	if !ok {
		return "$-1\r\n"
	}
	return fmt.Sprintf("$%d\r\n%s\r\n", len(e.value), e.value)
}

func (s *Server) set(args []string) string {
	if len(args) < 3 {
		return "-ERR wrong number of arguments for 'set' command\r\n"
	}
	key, value := args[1], []byte(args[2])
	nx := false
	var ttl time.Duration
	for i := 3; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "NX":
			nx = true
		case "PX":
			if i+1 >= len(args) {
				return "-ERR syntax error\r\n"
			}
			ms, err := strconv.Atoi(args[i+1])
			if err != nil || ms <= 0 {
				return "-ERR invalid expire time in 'set' command\r\n"
			}
			ttl = time.Duration(ms) * time.Millisecond
			i++
		default:
			return "-ERR syntax error\r\n"
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.live(key); nx && exists {
		return "$-1\r\n"
	}
	e := entry{value: value}
	if ttl > 0 {
		e.expires = s.now.Add(ttl)
	}
	s.data[key] = e
	return "+OK\r\n"
}

func (s *Server) del(args []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, k := range args[1:] {
		if _, ok := s.live(k); ok {
			delete(s.data, k)
			n++
		}
	}
	return fmt.Sprintf(":%d\r\n", n)
}

// readCommand reads one RESP array of bulk strings.
func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("unexpected %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil || n < 1 {
		return nil, fmt.Errorf("bad array length %q", line)
	}
	args := make([]string, 0, n)
	for range n {
		head, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(head, "$")))
		if err != nil || size < 0 {
			return nil, fmt.Errorf("bad bulk length %q", head)
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}
