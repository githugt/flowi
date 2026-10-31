// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// keepable is a transport that can be kept open: every Open makes a session
// that answers each message with "re:<body>", after a greeting.
type keepable struct {
	scripted
	mu       sync.Mutex
	opened   int
	sessions []*fakeSession
}

func (k *keepable) Open(_ context.Context, op transport.Operation) (transport.Session, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.opened++
	s := &fakeSession{op: op, inbox: transport.NewInbox(0)}
	s.inbox.Push(transport.Message{Direction: "received", Body: []byte("welcome")})
	k.sessions = append(k.sessions, s)
	return s, nil
}

type fakeSession struct {
	op     transport.Operation
	inbox  *transport.Inbox
	mu     sync.Mutex
	sent   []transport.Message
	closed int
}

func (s *fakeSession) Send(_ context.Context, m transport.Message) (transport.Message, error) {
	s.mu.Lock()
	s.sent = append(s.sent, m)
	s.mu.Unlock()
	s.inbox.Push(transport.Message{Direction: "received", Body: append([]byte("re:"), m.Body...)})
	m.Direction = "sent"
	return m, nil
}

func (s *fakeSession) Receive(ctx context.Context, r transport.Receive) ([]transport.Message, error) {
	return s.inbox.Wait(ctx, r)
}

func (s *fakeSession) Close() error {
	s.mu.Lock()
	s.closed++
	s.mu.Unlock()
	return nil
}

func (s *fakeSession) closes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// receivedBodies lists the bodies the step at path received, from its record.
func receivedBodies(events []Event, path string) []string {
	var out []string
	for _, ev := range events {
		if rc, ok := ev.(RecordCaptured); ok && rc.Path == path {
			for _, m := range rc.Record.Messages {
				if m.Direction == directionRecvd {
					out = append(out, m.Body.Text)
				}
			}
		}
	}
	return out
}

func sessionNode(id, req string, open, over string, closeAfter bool, needs ...string) model.Node {
	return model.Node{ID: id, Type: "request", Needs: needs, Request: &model.RequestCall{Request: model.Ref{ID: model.ID(req)}, Open: open, Over: over, Close: closeAfter}}
}

// chatFixture: "join" receives the greeting, "say" sends "hi" and waits for the reply.
func chatFixture(t *testing.T, nodes func(join, say model.Request) []model.Node) (*fixture, *keepable, model.Flow) {
	t.Helper()
	k := &keepable{}
	f := newFixture(t)
	f.transports = transport.Registry{"ws": k}
	join := f.addRequest("chat/join", asyncVersion(model.Async{Protocol: "ws", Server: "ws://chat", Receive: &model.AsyncReceive{Count: 1}}))
	say := f.addRequest("chat/say", asyncVersion(model.Async{Protocol: "ws", Server: "ws://chat",
		Send:    &model.AsyncSend{Body: model.Body{Text: "hi"}},
		Receive: &model.AsyncReceive{Count: 1}}))
	return f, k, f.addFlow("chat", nodes(join, say)...)
}

func TestRun_Given_a_connection_kept_open_Then_a_later_step_sends_and_receives_over_it(t *testing.T) {
	// Arrange
	f, k, fl := chatFixture(t, func(join, say model.Request) []model.Node {
		return []model.Node{
			sessionNode("join", string(join.ID), "room", "", false),
			sessionNode("say", string(say.ID), "", "room", true, "join"),
		}
	})

	// Act
	events, res := runFlow(t.Context(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if k.opened != 1 {
		t.Fatalf("the flow connected %d times, want once", k.opened)
	}
	s := k.sessions[0]
	if len(s.sent) != 1 || string(s.sent[0].Body) != "hi" {
		t.Fatalf("sent over the connection: %+v", s.sent)
	}
	if s.closes() != 1 {
		t.Errorf("closed %d times, want once (the last step said close)", s.closes())
	}
	if got := receivedBodies(events, "say"); len(got) != 1 || got[0] != "re:hi" {
		t.Errorf("the second step should have the reply, got %v", got)
	}
}

func TestRun_Given_a_connection_nobody_closes_Then_it_closes_when_the_run_ends(t *testing.T) {
	// Arrange
	f, k, fl := chatFixture(t, func(join, say model.Request) []model.Node {
		return []model.Node{
			sessionNode("join", string(join.ID), "room", "", false),
			sessionNode("say", string(say.ID), "", "room", false, "join"),
		}
	})

	// Act
	_, res := runFlow(t.Context(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status != StatusPassed || k.sessions[0].closes() != 1 {
		t.Fatalf("status %s, closed %d times: a connection must not outlive its run", res.Status, k.sessions[0].closes())
	}
}

func TestRun_Given_a_step_over_a_connection_nobody_opened_Then_it_fails_and_says_which(t *testing.T) {
	// Arrange
	f, _, fl := chatFixture(t, func(_, say model.Request) []model.Node {
		return []model.Node{sessionNode("say", string(say.ID), "", "room", false)}
	})

	// Act
	_, res := runFlow(t.Context(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status == StatusPassed || !hasCode(res.Diags, diag.CodeAsyncConn) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestRun_Given_two_steps_open_the_same_name_Then_the_second_fails_and_its_connection_is_closed(t *testing.T) {
	// Arrange
	f, k, fl := chatFixture(t, func(join, _ model.Request) []model.Node {
		return []model.Node{
			sessionNode("a", string(join.ID), "room", "", false),
			sessionNode("b", string(join.ID), "room", "", false, "a"),
		}
	})

	// Act
	_, res := runFlow(t.Context(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status == StatusPassed || !hasCode(res.Diags, diag.CodeAsyncConn) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if len(k.sessions) != 2 || k.sessions[1].closes() != 1 {
		t.Fatalf("the refused connection must be closed, sessions %d", len(k.sessions))
	}
}

func TestRun_Given_a_protocol_that_cannot_be_kept_open_Then_the_opening_step_says_so(t *testing.T) {
	// Arrange
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": &scripted{}}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Receive: &model.AsyncReceive{Count: 1}}))
	fl := f.addFlow("f", sessionNode("a", string(r.ID), "room", "", false))

	// Act
	_, res := runFlow(t.Context(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status == StatusPassed || !hasCode(res.Diags, diag.CodeAsyncConn) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestRun_Given_the_run_is_cancelled_Then_open_connections_are_closed(t *testing.T) {
	// Arrange
	f, k, fl := chatFixture(t, func(join, _ model.Request) []model.Node {
		return []model.Node{sessionNode("join", string(join.ID), "room", "", false)}
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Act
	_, _ = runFlow(ctx, t, f.deps(), f.runInput(fl))

	// Assert
	for i, s := range k.sessions {
		if s.closes() == 0 {
			t.Errorf("session %d was left open", i)
		}
	}
}
