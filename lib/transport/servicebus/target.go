// SPDX-License-Identifier: Apache-2.0

package servicebus

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const namespaceSuffix = ".servicebus.windows.net"

// entity is where a receive reads: a queue, or a topic's subscription.
type entity struct{ queue, topic, subscription string }

// sendTarget is the queue or topic a send goes to. A channel written
// topic/subscription (for a request that also receives) sends to the topic.
func sendTarget(channel string) (string, error) {
	parts, err := splitChannel(channel)
	if err != nil {
		return "", err
	}
	return parts[0], nil
}

// receiveTarget is what a receive reads: "orders" is a queue, "events/billing"
// the subscription billing of the topic events.
func receiveTarget(channel string) (entity, error) {
	parts, err := splitChannel(channel)
	if err != nil {
		return entity{}, err
	}
	if len(parts) == 2 {
		return entity{topic: parts[0], subscription: parts[1]}, nil
	}
	return entity{queue: parts[0]}, nil
}

func splitChannel(channel string) ([]string, error) {
	parts := strings.Split(channel, "/")
	if len(parts) > 2 {
		return nil, fmt.Errorf("channel %q must be a queue, a topic, or topic/subscription", channel)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("channel %q must be a queue, a topic, or topic/subscription", channel)
		}
	}
	return parts, nil
}

// namespaceOf is the fully qualified namespace a server names: the host,
// with an optional sb:// in front, or just the namespace name.
func namespaceOf(server string) (string, error) {
	host := strings.Trim(strings.TrimPrefix(server, "sb://"), "/")
	if host == "" || strings.Contains(host, "://") || strings.Contains(host, "/") {
		return "", errors.New("server must be <namespace>.servicebus.windows.net or sb://<namespace>.servicebus.windows.net")
	}
	if !strings.Contains(host, ".") {
		host += namespaceSuffix
	}
	return host, nil
}

// explicitConnection returns the connection string of the binding. Errors
// name the binding and never hold its value.
func explicitConnection(binding map[string]string) (string, error) {
	cs := strings.TrimSpace(binding["connection_string"])
	if cs == "" {
		return "", errors.New("auth is explicit but binding connection_string is empty")
	}
	if !strings.Contains(strings.ToLower(cs), "endpoint=") {
		return "", errors.New("binding connection_string is not a Service Bus connection string (Endpoint=sb://…;SharedAccessKeyName=…;SharedAccessKey=…)")
	}
	return cs, nil
}

// sendProperties are the binding options that become message properties.
type sendProperties struct {
	messageID, sessionID, subject string
	ttl                           time.Duration
}

// sendPropertiesOf reads session_id, subject, message_id and ttl. A message
// id is always set (generated when absent) so a request that also receives
// can tell its own message from a reply.
func sendPropertiesOf(binding map[string]string) (sendProperties, error) {
	p := sendProperties{messageID: binding["message_id"], sessionID: binding["session_id"], subject: binding["subject"]}
	if raw := binding["ttl"]; raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return p, fmt.Errorf("binding ttl %q must be a positive duration such as 90s or 10m", raw)
		}
		p.ttl = ttl
	}
	if p.messageID == "" {
		id, err := newMessageID()
		if err != nil {
			return p, err
		}
		p.messageID = id
	}
	return p, nil
}

func newMessageID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("making a message id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Read modes of a receive (binding read).
const (
	readLock = "lock" // peek-lock: kept messages are completed (removed), the others given back
	readPeek = "peek" // only look: nothing is locked, removed or counted as a delivery
)

// readModeOf reads binding read; absent means lock, which is what a request
// without the binding always did.
func readModeOf(binding map[string]string) (string, error) {
	switch mode := binding["read"]; mode {
	case "", readLock:
		return readLock, nil
	case readPeek:
		return readPeek, nil
	default:
		return "", fmt.Errorf("binding read %q must be %s or %s", mode, readPeek, readLock)
	}
}
