// SPDX-License-Identifier: Apache-2.0

package amqp10

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	amqp "github.com/Azure/go-amqp"

	"github.com/githugt/flowi/lib/transport"
)

// sendProperties are the message properties the binding sets.
type sendProperties struct {
	subject     string
	contentType string
	durable     bool
	ttl         time.Duration
}

// sendProps reads the binding names that describe the message to send.
func sendProps(op transport.Operation) (sendProperties, error) {
	p := sendProperties{subject: op.Binding["subject"], contentType: op.Binding["content_type"]}
	if v := op.Binding["durable"]; v != "" {
		d, err := strconv.ParseBool(v)
		if err != nil {
			return p, fmt.Errorf("binding durable must be true or false, got %q", v)
		}
		p.durable = d
	}
	if v := op.Binding["ttl"]; v != "" {
		ms, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return p, fmt.Errorf("binding ttl must be a number of milliseconds, got %q", v)
		}
		p.ttl = time.Duration(ms) * time.Millisecond
	}
	return p, nil
}

// message builds the message to send: the body as a data section, the message
// headers as application properties, the run's id as the message id.
func (r *run) message() *amqp.Message {
	m := amqp.NewMessage(r.op.Send.Body)
	m.Properties = &amqp.MessageProperties{MessageID: r.id}
	if r.props.subject != "" {
		m.Properties.Subject = &r.props.subject
	}
	if r.props.contentType != "" {
		m.Properties.ContentType = &r.props.contentType
	}
	if r.props.durable || r.props.ttl > 0 {
		m.Header = &amqp.MessageHeader{Durable: r.props.durable, TTL: r.props.ttl}
	}
	if len(r.op.Send.Headers) > 0 {
		m.ApplicationProperties = make(map[string]any, len(r.op.Send.Headers))
		for _, h := range r.op.Send.Headers {
			m.ApplicationProperties[h.Name] = h.Value
		}
	}
	return m
}

// isOwn reports whether m is the message this run sent: a queue that is both
// sent to and received from hands it back.
func (r *run) isOwn(m *amqp.Message) bool {
	return r.op.Send != nil && m.Properties != nil && m.Properties.MessageID == r.id
}

// received turns a delivered message into a transport message: the standard
// properties that are set and the application properties become headers.
func (r *run) received(m *amqp.Message) transport.Message {
	out := transport.Message{Direction: "received", At: r.op.Now(), Body: bodyOf(m)}
	if p := m.Properties; p != nil {
		add := func(name string, v any) {
			if s := text(v); s != "" {
				out.Headers = append(out.Headers, transport.Header{Name: name, Value: s})
			}
		}
		add("message-id", p.MessageID)
		add("correlation-id", p.CorrelationID)
		add("subject", deref(p.Subject))
		add("content-type", deref(p.ContentType))
		add("reply-to", deref(p.ReplyTo))
		add("to", deref(p.To))
	}
	for _, name := range sortedKeys(m.ApplicationProperties) {
		out.Headers = append(out.Headers, transport.Header{Name: name, Value: text(m.ApplicationProperties[name])})
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// text shows an AMQP property value as a string.
func text(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	}
	return fmt.Sprint(v)
}

// bodyOf is the message body: the data sections joined, else the AMQP value
// (a string or bytes as they are, anything else as JSON), else the sequences as JSON.
func bodyOf(m *amqp.Message) []byte {
	switch {
	case len(m.Data) > 0:
		return []byte(strings.Join(sectionsAsStrings(m.Data), ""))
	case m.Value != nil:
		return valueBytes(m.Value)
	case len(m.Sequence) > 0:
		return jsonBytes(m.Sequence)
	}
	return nil
}

func sectionsAsStrings(d [][]byte) []string {
	out := make([]string, len(d))
	for i, s := range d {
		out[i] = string(s)
	}
	return out
}

func valueBytes(v any) []byte {
	switch t := v.(type) {
	case string:
		return []byte(t)
	case []byte:
		return t
	}
	return jsonBytes(v)
}

func jsonBytes(v any) []byte {
	if data, err := json.Marshal(v); err == nil {
		return data
	}
	return []byte(fmt.Sprint(v))
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
