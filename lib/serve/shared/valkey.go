// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout = 2 * time.Second
	// quietPeriod is how long after a failure calls return ErrUnavailable
	// without touching the network.
	quietPeriod = 15 * time.Second
	// maxReply bounds a reply line or bulk string read from the server.
	maxReply = maxValue + 1024
)

// ValkeyConfig configures a Valkey store.
type ValkeyConfig struct {
	Addr     string // host:port
	Password string // sent with AUTH when not empty; never logged
	TLS      bool
	Prefix   string        // put before every key, so workspaces do not meet
	Timeout  time.Duration // per connect and per command; default 2s
	Logger   *slog.Logger
	Now      func() time.Time // for the quiet period; default time.Now
}

// Valkey is a Store on a Valkey or Redis server, over RESP2.
//
// It keeps one connection, opened on first use and reopened once if a call
// finds it closed. After a failure that a fresh connection does not cure
// it returns ErrUnavailable for quietPeriod without dialing.
type Valkey struct {
	cfg ValkeyConfig

	mu        sync.Mutex // guards conn, rd and downUntil; also serialises commands
	conn      net.Conn
	rd        *bufio.Reader
	downUntil time.Time
}

// NewValkey checks the configuration. It does not connect: a server that
// is down at start must not stop serve from starting.
func NewValkey(cfg ValkeyConfig) (*Valkey, error) {
	if cfg.Addr == "" {
		return nil, errors.New("shared: no Valkey address")
	}
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return nil, fmt.Errorf("shared: Valkey address %q is not host:port", cfg.Addr)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Valkey{cfg: cfg}, nil
}

// Claim implements Store with SET key 1 NX PX ttl.
func (v *Valkey) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	r, err := v.do(ctx, "SET", v.cfg.Prefix+key, "1", "NX", "PX", millis(ttl))
	if err != nil {
		return false, err
	}
	return r.kind == replyStatus, nil // "+OK" when set, a null bulk when the key existed
}

// Put implements Store with SET key value PX ttl.
func (v *Valkey) Put(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if len(value) > maxValue {
		return ErrTooLarge
	}
	_, err := v.do(ctx, "SET", v.cfg.Prefix+key, string(value), "PX", millis(ttl))
	return err
}

// Get implements Store with GET.
func (v *Valkey) Get(ctx context.Context, key string) ([]byte, bool, error) {
	r, err := v.do(ctx, "GET", v.cfg.Prefix+key)
	if err != nil {
		return nil, false, err
	}
	if r.kind != replyBulk {
		return nil, false, nil
	}
	return r.bulk, true, nil
}

// Close drops the connection.
func (v *Valkey) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.drop()
	return nil
}

func millis(d time.Duration) string { return strconv.FormatInt(max(d.Milliseconds(), 1), 10) }

type replyKind int

const (
	replyStatus replyKind = iota + 1
	replyBulk
	replyNull
	replyInt
)

type reply struct {
	kind replyKind
	bulk []byte
}

// serverError is an error reply from the server; the connection is fine.
type serverError string

func (e serverError) Error() string { return "the server answered: " + string(e) }

// do sends one command. A closed idle connection is replaced once; any
// other failure starts the quiet period.
func (v *Valkey) do(ctx context.Context, args ...string) (reply, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cfg.Now().Before(v.downUntil) {
		return reply{}, fmt.Errorf("%w (retrying after a failure)", ErrUnavailable)
	}
	for attempt := range 2 {
		reused := v.conn != nil
		if !reused {
			if err := v.connect(ctx); err != nil {
				return reply{}, v.fail(err)
			}
		}
		r, err := v.roundTrip(ctx, args)
		var srvErr serverError
		switch {
		case err == nil:
			return r, nil
		case errors.As(err, &srvErr):
			return reply{}, err
		}
		v.drop()
		if !reused || attempt == 1 {
			return reply{}, v.fail(err)
		}
	}
	return reply{}, v.fail(errors.New("no connection"))
}

// fail starts the quiet period and wraps the cause in ErrUnavailable.
func (v *Valkey) fail(cause error) error {
	v.downUntil = v.cfg.Now().Add(quietPeriod)
	v.cfg.Logger.Warn("shared store unavailable", "Event", "SharedStoreUnavailable", "Addr", v.cfg.Addr, "Error", cause.Error(), "RetryAfter", quietPeriod)
	return fmt.Errorf("%w: %w", ErrUnavailable, cause)
}

func (v *Valkey) drop() {
	if v.conn != nil {
		_ = v.conn.Close() // the connection is being discarded; nothing depends on a clean close
	}
	v.conn, v.rd = nil, nil
}

func (v *Valkey) connect(ctx context.Context) error {
	d := &net.Dialer{Timeout: v.cfg.Timeout}
	conn, err := d.DialContext(ctx, "tcp", v.cfg.Addr)
	if err != nil {
		return err
	}
	if v.cfg.TLS {
		host, _, _ := net.SplitHostPort(v.cfg.Addr)
		conn = tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	}
	v.conn, v.rd = conn, bufio.NewReaderSize(conn, 4096)
	if v.cfg.Password == "" {
		return nil
	}
	if _, err := v.roundTrip(ctx, []string{"AUTH", v.cfg.Password}); err != nil {
		v.drop()
		return redactPassword(err, v.cfg.Password)
	}
	return nil
}

// redactPassword makes sure an error cannot repeat the password.
func redactPassword(err error, password string) error {
	if password != "" && strings.Contains(err.Error(), password) {
		return errors.New("authentication failed")
	}
	return err
}

func (v *Valkey) roundTrip(ctx context.Context, args []string) (reply, error) {
	deadline := time.Now().Add(v.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := v.conn.SetDeadline(deadline); err != nil {
		return reply{}, err
	}
	if _, err := io.WriteString(v.conn, encodeCommand(args)); err != nil {
		return reply{}, err
	}
	return readReply(v.rd)
}

// encodeCommand writes a command as a RESP array of bulk strings, which is
// length-prefixed, so no argument can inject another command.
func encodeCommand(args []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.String()
}

func readReply(r *bufio.Reader) (reply, error) {
	line, err := readLine(r)
	if err != nil {
		return reply{}, err
	}
	if line == "" {
		return reply{}, errors.New("empty reply")
	}
	body := line[1:]
	switch line[0] {
	case '+':
		return reply{kind: replyStatus}, nil
	case '-':
		return reply{}, serverError(body)
	case ':':
		return reply{kind: replyInt}, nil
	case '$':
		return readBulk(r, body)
	}
	return reply{}, fmt.Errorf("unexpected reply type %q", line[0])
}

func readBulk(r *bufio.Reader, lengthText string) (reply, error) {
	n, err := strconv.Atoi(lengthText)
	if err != nil || n < -1 || n > maxReply {
		return reply{}, errors.New("bad bulk length")
	}
	if n == -1 {
		return reply{kind: replyNull}, nil
	}
	buf := make([]byte, n+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return reply{}, err
	}
	return reply{kind: replyBulk, bulk: buf[:n]}, nil
}

// readLine reads up to CRLF, bounded so a hostile server cannot make the
// client buffer without end.
func readLine(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		chunk, err := r.ReadSlice('\n')
		b.Write(chunk)
		if b.Len() > maxReply {
			return "", errors.New("reply line too long")
		}
		if err == nil {
			break
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return "", err
		}
	}
	return strings.TrimRight(b.String(), "\r\n"), nil
}
