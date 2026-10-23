// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNoEndpoint is returned when no telemetry_endpoint is configured: a
// build or installation without an endpoint sends nothing.
var ErrNoEndpoint = errors.New("telemetry: no endpoint configured")

const sendTimeout = 5 * time.Second

// Sender posts a payload to the configured endpoint.
type Sender struct {
	endpoint string
	client   *http.Client
}

// NewSender returns a sender for endpoint; a nil client gets a timeout of
// its own.
func NewSender(endpoint string, client *http.Client) *Sender {
	if client == nil {
		client = &http.Client{Timeout: sendTimeout}
	}
	return &Sender{endpoint: endpoint, client: client}
}

// Send validates and posts p once, without retries. A report that does not
// arrive is simply lost.
func (s *Sender) Send(ctx context.Context, p Payload) error {
	if s.endpoint == "" {
		return ErrNoEndpoint
	}
	if err := p.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encoding the report: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building the report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending the report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // the answer is not needed past its status
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sending the report: server answered %d", resp.StatusCode)
	}
	return nil
}
