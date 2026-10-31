// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// Run calls the method and returns what came back. All four kinds of method
// work. A method that streams from the client takes a JSON array as its body,
// one message for each element (a lone object is one message); a stream from
// the server is collected with the same rules as an async receive (count,
// timeout, match, live OnMessage).
//
// Goroutine ownership: a server stream starts one reader goroutine, and a
// bidirectional call one more that sends; Run cancels the call and waits for
// both before it returns, and closes the connection last.
func Run(ctx context.Context, c Call) (Result, error) {
	started := c.now()
	if c.Web {
		return runWeb(ctx, c, started)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	conn, err := dial(c)
	if err != nil {
		return Result{}, err
	}
	defer conn.Close() //nolint:errcheck // nothing useful to do if closing fails
	md, err := resolveMethod(ctx, conn, c)
	if err != nil {
		return Result{}, err
	}
	in, warnings, err := requestMessages(md, c.JSON)
	if err != nil {
		return Result{}, err
	}
	out, err := outgoingContext(ctx, c.Metadata)
	if err != nil {
		return Result{}, err
	}
	res := Result{Connect: c.now().Sub(started), Warnings: warnings}
	if res.Messages, err = sentMessages(in, c); err != nil {
		return res, err
	}
	path := "/" + string(md.Parent().FullName()) + "/" + string(md.Name())
	switch {
	case md.IsStreamingServer():
		err = runStream(out, conn, path, md, in, c, &res)
	case md.IsStreamingClient():
		runClientStream(out, conn, path, md, in, c, &res)
	default:
		runUnary(out, conn, path, md, in[0], c, &res)
	}
	res.Total = c.now().Sub(started)
	return res, err
}

func (c Call) sentHeaders() []transport.Header { return append([]transport.Header(nil), c.Metadata...) }

// dial makes the connection (lazily: nothing is sent until the first call).
func dial(c Call) (*grpc.ClientConn, error) {
	hostport, useTLS := model.GRPCTarget(c.Server)
	if hostport == "" {
		return nil, errors.New("a gRPC call needs a server")
	}
	creds := credentials.TransportCredentials(insecure.NewCredentials())
	if useTLS {
		cfg := c.TLS
		if cfg == nil {
			cfg = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		creds = credentials.NewTLS(cfg)
	}
	conn, err := grpc.NewClient(hostport, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", hostport, err)
	}
	return conn, nil
}

// outgoingContext attaches the metadata: names lower-cased. The value of a
// binary (-bin) header is written as base64 and sent as the bytes it stands for.
func outgoingContext(ctx context.Context, headers []transport.Header) (context.Context, error) {
	if len(headers) == 0 {
		return ctx, nil
	}
	md := metadata.MD{}
	for _, h := range headers {
		name := strings.ToLower(h.Name)
		value := h.Value
		if strings.HasSuffix(name, "-bin") {
			raw, err := decodeBase64(value)
			if err != nil {
				return nil, fmt.Errorf("metadata %q: a binary (-bin) value is written as base64: %w", h.Name, err)
			}
			value = string(raw)
		}
		md.Append(name, value)
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}

// sentMessages records the requests as sent. The metadata went with the call,
// so only the first message carries it.
func sentMessages(in []*dynamicpb.Message, c Call) ([]transport.Message, error) {
	out := make([]transport.Message, 0, len(in))
	for i, m := range in {
		body, err := jsonOf(m)
		if err != nil {
			return nil, err
		}
		msg := transport.Message{Direction: "sent", At: c.now(), Body: body}
		if i == 0 {
			msg.Headers = c.sentHeaders()
		}
		out = append(out, msg)
	}
	return out, nil
}

// requestMessages converts the JSON into the messages to send: for a method
// that streams from the client, a JSON array is one message for each element
// (an empty array sends none); anything else, and every other kind of method,
// is exactly one message.
func requestMessages(md protoreflect.MethodDescriptor, body []byte) ([]*dynamicpb.Message, []string, error) {
	trimmed := bytes.TrimSpace(body)
	if !md.IsStreamingClient() || !bytes.HasPrefix(trimmed, []byte("[")) {
		in, warnings, err := requestMessage(md, body)
		if err != nil {
			return nil, nil, err
		}
		return []*dynamicpb.Message{in}, warnings, nil
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(trimmed, &elements); err != nil {
		return nil, nil, &BadBodyError{Detail: "the array of messages is not valid JSON: " + err.Error()}
	}
	var out []*dynamicpb.Message
	var warnings []string
	for i, el := range elements {
		in, w, err := requestMessage(md, el)
		var bad *BadBodyError
		if errors.As(err, &bad) {
			return nil, nil, &BadBodyError{Detail: fmt.Sprintf("message %d: %s", i+1, bad.Detail)}
		}
		if err != nil {
			return nil, nil, err
		}
		out = append(out, in)
		for _, text := range w {
			warnings = append(warnings, fmt.Sprintf("message %d: %s", i+1, text))
		}
	}
	return out, warnings, nil
}

// requestMessage converts the JSON into the method's request message. A
// field the message does not have is left out with a warning (a request
// written for one method and pointed at another still goes); any other way the
// body does not fit is a BadBodyError.
func requestMessage(md protoreflect.MethodDescriptor, body []byte) (*dynamicpb.Message, []string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}")
	}
	in := dynamicpb.NewMessage(md.Input())
	err := (protojson.UnmarshalOptions{}).Unmarshal(body, in)
	if err == nil {
		return in, nil, nil
	}
	if !strings.Contains(err.Error(), "unknown field") {
		return nil, nil, &BadBodyError{Detail: strings.TrimPrefix(err.Error(), "proto: ")}
	}
	in = dynamicpb.NewMessage(md.Input())
	if lenient := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, in); lenient != nil {
		return nil, nil, &BadBodyError{Detail: strings.TrimPrefix(lenient.Error(), "proto: ")}
	}
	warning := fmt.Sprintf("%s is not a field of %s: it was left out (%s)", unknownField(err), md.Input().FullName(), strings.TrimPrefix(err.Error(), "proto: "))
	return in, []string{warning}, nil
}

// unknownField is the field name in a protojson "unknown field" error.
func unknownField(err error) string {
	text := err.Error()
	if i := strings.Index(text, "unknown field"); i >= 0 {
		return strings.TrimSpace(text[i+len("unknown field"):])
	}
	return "a field"
}

// jsonOf is a message as compact JSON with the proto field names, zero values
// included, so an expression can read any field. protojson randomizes its
// spacing on purpose, so the output is compacted to be stable.
func jsonOf(m *dynamicpb.Message) ([]byte, error) {
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("writing a message as JSON: %w", err)
	}
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return nil, fmt.Errorf("writing a message as JSON: %w", err)
	}
	return out.Bytes(), nil
}

// runUnary makes the call and puts the response or the status in res.
func runUnary(ctx context.Context, conn *grpc.ClientConn, path string, md protoreflect.MethodDescriptor, in *dynamicpb.Message, c Call, res *Result) {
	reply := dynamicpb.NewMessage(md.Output())
	var header, trailer metadata.MD
	err := conn.Invoke(ctx, path, in, reply, grpc.Header(&header), grpc.Trailer(&trailer))
	res.Header, res.Trailer = headersOf(header), headersOf(trailer)
	res.Code, res.Message = statusOf(err)
	if err == nil {
		if body, jerr := jsonOf(reply); jerr == nil {
			res.Messages = append(res.Messages, transport.Message{Direction: "received", At: c.now(), Body: body, Headers: res.Header})
		}
	}
}

// statusOf is the gRPC code and message of a call's error (OK for none). A
// cancelled or timed-out context is Canceled or DeadlineExceeded.
func statusOf(err error) (codes.Code, string) {
	if err == nil {
		return codes.OK, ""
	}
	st, _ := status.FromError(err)
	return st.Code(), st.Message()
}

// decodeBase64 reads standard or URL-safe base64, padded or not.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	if raw, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

// headersOf lists metadata in name order; the value of a binary (-bin) header
// is shown as base64, as the bytes are not text.
func headersOf(md metadata.MD) []transport.Header {
	names := make([]string, 0, len(md))
	for name := range md {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []transport.Header
	for _, name := range names {
		for _, v := range md[name] {
			if strings.HasSuffix(name, "-bin") {
				v = base64.StdEncoding.EncodeToString([]byte(v))
			}
			out = append(out, transport.Header{Name: name, Value: v})
		}
	}
	return out
}

// errStreamEnded tells the collector the server closed the stream: nothing
// more will come, so waiting for more messages is pointless.
var errStreamEnded = errors.New("the stream ended")

// runClientStream sends the messages and reads the one response.
func runClientStream(ctx context.Context, conn *grpc.ClientConn, path string, md protoreflect.MethodDescriptor, in []*dynamicpb.Message, c Call, res *Result) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true}, path)
	if err != nil {
		res.Code, res.Message = statusOf(err)
		return
	}
	if err := sendAll(stream, in); err != nil {
		res.Code, res.Message = statusOf(err)
		return
	}
	reply := dynamicpb.NewMessage(md.Output())
	err = stream.RecvMsg(reply)
	if h, herr := stream.Header(); herr == nil {
		res.Header = headersOf(h)
	}
	res.Trailer = headersOf(stream.Trailer())
	res.Code, res.Message = statusOf(err)
	if err != nil {
		return
	}
	if body, jerr := jsonOf(reply); jerr == nil {
		res.Messages = append(res.Messages, transport.Message{Direction: "received", At: c.now(), Body: body, Headers: res.Header})
	}
}

// sendAll sends the messages and closes the sending side. When the server
// ends the call early, SendMsg reports io.EOF and the real status is read by
// the receiver, so that is not an error here.
func sendAll(stream grpc.ClientStream, in []*dynamicpb.Message) error {
	for _, m := range in {
		if err := stream.SendMsg(m); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	return stream.CloseSend()
}

// runStream sends the request or requests and collects the stream. A call that
// streams both ways sends from its own goroutine, so a server that answers
// while the requests are still going out is read at the same time.
func runStream(ctx context.Context, conn *grpc.ClientConn, path string, md protoreflect.MethodDescriptor, in []*dynamicpb.Message, c Call, res *Result) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: md.IsStreamingClient()}, path)
	if err != nil {
		res.Code, res.Message = statusOf(err)
		return nil
	}
	if !md.IsStreamingClient() {
		if err := sendAll(stream, in); err != nil {
			res.Code, res.Message = statusOf(err)
			return nil
		}
	}
	// Without a count to wait for, a stream is read until the server ends it, the
	// timeout passes or the call is cancelled, and the messages that came are the
	// result (a watch never ends by itself).
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
	incoming := make(chan transport.Message)
	fail := make(chan error, 2)
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		readStream(ctx, stream, md, c.now, incoming, fail)
	}()
	if md.IsStreamingClient() {
		done.Add(1)
		go func() {
			defer done.Done()
			if err := sendAll(stream, in); err != nil {
				fail <- err
			}
		}()
	}

	kept, werr := transport.Wait(ctx, rc, incoming, fail)
	cancel()
	done.Wait()
	res.Messages = append(res.Messages, kept...)
	if h, herr := stream.Header(); herr == nil {
		res.Header = headersOf(h)
	}
	res.Trailer = headersOf(stream.Trailer())
	return streamOutcome(werr, res, unbounded)
}

// readStream feeds the messages of the stream to incoming until it ends; the
// end or a failure goes to fail.
func readStream(ctx context.Context, stream grpc.ClientStream, md protoreflect.MethodDescriptor, now func() time.Time, incoming chan<- transport.Message, fail chan<- error) {
	for {
		reply := dynamicpb.NewMessage(md.Output())
		if err := stream.RecvMsg(reply); err != nil {
			if errors.Is(err, io.EOF) {
				fail <- errStreamEnded
			} else {
				fail <- err
			}
			return
		}
		body, err := jsonOf(reply)
		if err != nil {
			fail <- err
			return
		}
		select {
		case incoming <- transport.Message{Direction: "received", At: now(), Body: body}:
		case <-ctx.Done():
			return
		}
	}
}

// timeIsUp says the wait ended because its time ran out. The call's deadline and the wait's timer
// are the same length, so either can fire first: the wait reports ErrTimeout, the call reports
// context.DeadlineExceeded or, once the stream notices, a gRPC DeadlineExceeded status.
func timeIsUp(err error) bool {
	return errors.Is(err, transport.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded
}

// streamOutcome turns how the wait ended into the result's status and the
// error to return: the end of the stream is success, a timeout is
// transport.ErrTimeout (with the partial messages), a gRPC failure is the code.
func streamOutcome(err error, res *Result, unbounded bool) error {
	switch {
	case err == nil, errors.Is(err, errStreamEnded):
		res.Code = codes.OK
		return nil
	case unbounded && timeIsUp(err):
		res.Code = codes.OK // nothing was waited for: the time is up, and what came is the result
		return nil
	case errors.Is(err, transport.ErrTimeout), errors.Is(err, transport.ErrTooLarge):
		res.Code = codes.OK
		return err
	}
	if st, ok := status.FromError(err); ok {
		res.Code, res.Message = st.Code(), st.Message()
		return nil
	}
	if errors.Is(err, context.Canceled) {
		res.Code = codes.Canceled
		return nil
	}
	return err
}
