// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/transport"
)

// gRPC-Web is gRPC for browsers and proxies that cannot do HTTP/2 trailers: the
// same messages, framed the same way, in an ordinary HTTP POST, with the status
// in a last frame of the body. Unary and server-streaming methods work; a
// method that streams from the client does not exist in gRPC-Web.
const (
	webContentType = "application/grpc-web+proto"
	// webTrailerFlag marks the frame that holds the trailers, not a message.
	webTrailerFlag = 0x80
	// webCompressedFlag marks a message compressed by the sender.
	webCompressedFlag = 0x01
	// webMaxFrame bounds one frame a server may send.
	webMaxFrame = 64 << 20
)

// errNoWebStatus says the response body ended without a trailers frame.
var errNoWebStatus = errors.New("the response ended without a gRPC status")

// runWeb makes the call over gRPC-Web. Its schema is the .proto files of the
// call: gRPC-Web proxies do not offer reflection.
func runWeb(ctx context.Context, c Call, started time.Time) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	if len(c.Protos) == 0 {
		return Result{}, &SchemaError{Detail: "a gRPC-Web call needs the service's .proto files: its proxy does not offer reflection"}
	}
	md, err := resolveMethod(ctx, nil, c) // with files, the connection is never used
	if err != nil {
		return Result{}, err
	}
	if md.IsStreamingClient() {
		return Result{}, &SchemaError{Detail: fmt.Sprintf("%s streams from the client, which gRPC-Web has no way to do", md.FullName())}
	}
	in, warnings, err := requestMessages(md, c.JSON)
	if err != nil {
		return Result{}, err
	}
	res := Result{Connect: c.now().Sub(started), Warnings: warnings}
	if res.Messages, err = sentMessages(in, c); err != nil {
		return res, err
	}
	req, err := webRequest(ctx, c, md, in[0])
	if err != nil {
		return res, err
	}
	err = doWeb(ctx, req, c, md, &res)
	res.Total = c.now().Sub(started)
	return res, err
}

// webURL is the address of the method: the server's base address and the
// method path. A server written grpc:// or grpcs:// means http or https.
func webURL(c Call, md protoreflect.MethodDescriptor) (string, error) {
	server := strings.TrimSpace(c.Server)
	if server == "" {
		return "", errors.New("a gRPC call needs a server")
	}
	if !strings.Contains(server, "://") {
		scheme := "https"
		if _, useTLS := model.GRPCTarget(server); !useTLS {
			scheme = "http"
		}
		server = scheme + "://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return "", fmt.Errorf("the server %q is not an address: %w", c.Server, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "grpc":
		u.Scheme = "http"
	case "grpcs":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("a gRPC-Web server is an http:// or https:// address, not %s://", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + string(md.Parent().FullName()) + "/" + string(md.Name())
	return u.String(), nil
}

// webRequest builds the POST: the message in one frame, the metadata as headers.
func webRequest(ctx context.Context, c Call, md protoreflect.MethodDescriptor, in *dynamicpb.Message) (*http.Request, error) {
	target, err := webURL(c, md)
	if err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("writing the message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(webFrame(0, payload)))
	if err != nil {
		return nil, fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Content-Type", webContentType)
	req.Header.Set("Accept", webContentType)
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("Grpc-Timeout", strconv.FormatInt(c.timeout().Milliseconds(), 10)+"m")
	for _, h := range c.Metadata {
		req.Header.Add(strings.ToLower(h.Name), h.Value) // a -bin value is base64 in a header already
	}
	return req, nil
}

func webFrame(flag byte, payload []byte) []byte {
	out := make([]byte, 5, 5+len(payload))
	out[0] = flag
	binary.BigEndian.PutUint32(out[1:], uint32(len(payload))) //nolint:gosec // a message is far smaller than 4 GiB
	return append(out, payload...)
}

// doWeb sends the request and reads the frames of the answer into res.
func doWeb(ctx context.Context, req *http.Request, c Call, md protoreflect.MethodDescriptor, res *Result) error {
	transportCfg := http.DefaultTransport.(*http.Transport).Clone()
	transportCfg.TLSClientConfig = tlsOr(c.TLS)
	client := &http.Client{Transport: transportCfg}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		res.Code, res.Message = codes.Unavailable, webError(err)
		return nil
	}
	defer resp.Body.Close()
	res.Header = webHeaders(resp.Header)
	if resp.StatusCode != http.StatusOK {
		res.Code, res.Message = codeOfHTTP(resp.StatusCode), resp.Status
		return nil
	}
	return readWeb(ctx, resp, c, md, res)
}

func tlsOr(cfg *tls.Config) *tls.Config {
	if cfg != nil {
		return cfg
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

// webError words a failed request without the URL the Go client would quote.
func webError(err error) string {
	var inner *url.Error
	if errors.As(err, &inner) {
		err = inner.Err
	}
	return err.Error()
}

// codeOfHTTP maps an HTTP status the proxy answered to the gRPC code (the
// mapping of the gRPC over HTTP/2 specification).
func codeOfHTTP(status int) codes.Code {
	switch status {
	case http.StatusBadRequest:
		return codes.Internal
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.Unimplemented
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return codes.Unavailable
	}
	return codes.Unknown
}

func webHeaders(h http.Header) []transport.Header {
	var out []transport.Header
	for _, name := range sortedNames(h) {
		for _, v := range h[name] {
			out = append(out, transport.Header{Name: strings.ToLower(name), Value: v})
		}
	}
	return out
}

func sortedNames(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// readWeb reads the frames until the trailers, applying the same rules as a
// server stream (count, timeout, live messages), then sets the status.
func readWeb(ctx context.Context, resp *http.Response, c Call, md protoreflect.MethodDescriptor, res *Result) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	rc, unbounded := webReceive(c)
	incoming := make(chan transport.Message)
	fail := make(chan error, 1)
	var trailer trailerBox
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		readWebFrames(ctx, resp.Body, md, c.now, incoming, fail, &trailer)
	}()
	kept, werr := transport.Wait(ctx, rc, incoming, fail)
	cancel()
	_ = resp.Body.Close() // unblocks the reader if it waits on the network
	done.Wait()
	res.Messages = append(res.Messages, kept...)
	res.Trailer = trailer.get()
	err := streamOutcome(werr, res, unbounded)
	if errors.Is(werr, errNoWebStatus) {
		err = nil // the call has no trusted status, which is what its result says
	}
	applyWebStatus(res, werr, resp.Header)
	return err
}

// webReceive is how many messages to wait for and for how long, as for a
// gRPC stream: the whole stream unless the call says otherwise.
func webReceive(c Call) (transport.Receive, bool) {
	rc := transport.Receive{Count: math.MaxInt32, Timeout: c.timeout()}
	unbounded := c.Receive == nil || c.Receive.Count <= 0
	if c.Receive != nil {
		rc = *c.Receive
		if rc.Count <= 0 {
			rc.Count = math.MaxInt32
		}
		if rc.Timeout <= 0 {
			rc.Timeout = c.timeout()
		}
	}
	return rc, unbounded
}

// applyWebStatus reads grpc-status and grpc-message from the trailers (or, for
// a response that is only trailers, from the headers). Without a status the
// call has no outcome to trust.
func applyWebStatus(res *Result, waitErr error, header http.Header) {
	if code, message, ok := webStatus(res.Trailer); ok {
		res.Code, res.Message = code, message
		return
	}
	if code, message, ok := webStatus(webHeaders(header)); ok {
		res.Code, res.Message = code, message
		return
	}
	if errors.Is(waitErr, errNoWebStatus) {
		res.Code, res.Message = codes.Unknown, errNoWebStatus.Error()
	}
}

func webStatus(headers []transport.Header) (codes.Code, string, bool) {
	var code, message string
	found := false
	for _, h := range headers {
		switch strings.ToLower(h.Name) {
		case "grpc-status":
			code, found = h.Value, true
		case "grpc-message":
			message = h.Value
		}
	}
	n, err := strconv.Atoi(strings.TrimSpace(code))
	if !found || err != nil || n < 0 {
		return 0, "", false
	}
	if decoded, err := url.PathUnescape(message); err == nil {
		message = decoded
	}
	return codes.Code(n), message, true //nolint:gosec // a status code is small
}

// trailerBox hands the trailers from the reader goroutine to the caller.
type trailerBox struct {
	mu      sync.Mutex
	headers []transport.Header
}

func (b *trailerBox) set(h []transport.Header) { b.mu.Lock(); b.headers = h; b.mu.Unlock() }
func (b *trailerBox) get() []transport.Header  { b.mu.Lock(); defer b.mu.Unlock(); return b.headers }

// readWebFrames feeds the messages to incoming. The trailers frame ends the
// stream; so does a body that stops, as a failure.
func readWebFrames(ctx context.Context, body io.Reader, md protoreflect.MethodDescriptor, now func() time.Time, incoming chan<- transport.Message, fail chan<- error, trailer *trailerBox) {
	for {
		flag, payload, err := readFrame(body)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = errNoWebStatus
			}
			fail <- err
			return
		}
		if flag&webTrailerFlag != 0 {
			trailer.set(parseTrailers(payload))
			fail <- errStreamEnded
			return
		}
		if flag&webCompressedFlag != 0 {
			fail <- errors.New("the server sent a compressed message, which is not supported")
			return
		}
		reply := dynamicpb.NewMessage(md.Output())
		if err := proto.Unmarshal(payload, reply); err != nil {
			fail <- fmt.Errorf("reading a message of %s: %w", md.Output().FullName(), err)
			return
		}
		text, err := jsonOf(reply)
		if err != nil {
			fail <- err
			return
		}
		select {
		case incoming <- transport.Message{Direction: "received", At: now(), Body: text}:
		case <-ctx.Done():
			return
		}
	}
}

// readFrame reads one frame: a flag byte, a four-byte length and the payload.
func readFrame(r io.Reader) (flag byte, payload []byte, err error) {
	var head [5]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, nil, errors.New("the response stopped in the middle of a frame")
		}
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(head[1:])
	if n > webMaxFrame {
		return 0, nil, fmt.Errorf("a frame of %d bytes is more than the %d allowed", n, webMaxFrame)
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, errors.New("the response stopped in the middle of a frame")
	}
	return head[0], payload, nil
}

// parseTrailers reads the trailers frame: header lines, "name: value".
func parseTrailers(payload []byte) []transport.Header {
	var out []transport.Header
	for _, line := range strings.Split(string(payload), "\r\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, transport.Header{Name: strings.ToLower(strings.TrimSpace(name)), Value: strings.TrimSpace(value)})
	}
	return out
}
