// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/transport"
)

// Open connects and keeps the connection open for later steps.
//
// Goroutine ownership: Open starts one reader goroutine that pushes frames
// into the session's inbox; Close stops it and waits. ctx bounds only the
// handshake: the connection outlives the step that opened it.
func (Transport) Open(ctx context.Context, op transport.Operation) (transport.Session, error) {
	conn, err := dial(ctx, op)
	if err != nil {
		return nil, err
	}
	readCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &session{conn: conn, inbox: transport.NewInbox(0), now: op.Now, cancel: cancel}
	s.wg.Add(1)
	go s.read(readCtx)
	return s, nil
}

var _ transport.Opener = Transport{}

type session struct {
	conn   *websocket.Conn
	inbox  *transport.Inbox
	now    func() time.Time
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
	sendMu sync.Mutex
}

func (s *session) read(ctx context.Context) {
	defer s.wg.Done()
	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.inbox.Fail(fmt.Errorf("receiving: %w", err))
			}
			return
		}
		s.inbox.Push(transport.Message{Direction: "received", At: s.now(), Body: data})
	}
}

func (s *session) Send(ctx context.Context, m transport.Message) (transport.Message, error) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	kind := websocket.MessageBinary
	if transport.IsText(m.Body) {
		kind = websocket.MessageText
	}
	if err := s.conn.Write(ctx, kind, m.Body); err != nil {
		return m, fmt.Errorf("sending: %w", err)
	}
	m.Direction, m.At = "sent", s.now()
	return m, nil
}

func (s *session) Receive(ctx context.Context, r transport.Receive) ([]transport.Message, error) {
	return s.inbox.Wait(ctx, r)
}

func (s *session) Close() error {
	s.once.Do(func() {
		s.cancel()
		_ = s.conn.Close(websocket.StatusNormalClosure, "") // best-effort goodbye; the run is over either way
		_ = s.conn.CloseNow()
		s.wg.Wait()
	})
	return nil
}
