// SPDX-License-Identifier: Apache-2.0

package sqs_test

import (
	"crypto/md5" //nolint:gosec // SQS defines MD5OfMessageBody; the SDK checks it
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSQS is an in-process stand-in for the few SQS calls the transport makes
// (AWS JSON 1.0 over HTTP). Visibility is modelled with real time; a received
// message stays hidden for its visibility timeout.
type fakeSQS struct {
	URL string

	mu     sync.Mutex
	queues map[string]*fakeQueue
	calls  []string // operation names, in order
	keyIDs []string // access key ids the requests were signed with
	next   int
}

type fakeQueue struct{ msgs []*fakeMsg }

type fakeMsg struct {
	id, handle, body, group, dedup string
	delay                          float64
	attrs                          map[string]string
	visibleAt                      time.Time
	receives                       int
	deleted                        bool
}

var credentialRe = regexp.MustCompile(`Credential=([^/]+)/`)

func startFakeSQS(t *testing.T, queues ...string) *fakeSQS {
	t.Helper()
	f := &fakeSQS{queues: map[string]*fakeQueue{}}
	for _, q := range queues {
		f.queues[q] = &fakeQueue{}
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

func (f *fakeSQS) queueURL(name string) string { return f.URL + "/000000000000/" + name }

// put adds a message that is visible at once.
func (f *fakeSQS) put(queue, body string) *fakeMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enqueue(queue, body)
}

func (f *fakeSQS) enqueue(queue, body string) *fakeMsg {
	f.next++
	m := &fakeMsg{id: fmt.Sprintf("msg-%d", f.next), body: body, attrs: map[string]string{}}
	f.queues[queue].msgs = append(f.queues[queue].msgs, m)
	return m
}

// snapshot returns copies of the queue's messages that were not deleted.
func (f *fakeSQS) snapshot(queue string) []fakeMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeMsg
	for _, m := range f.queues[queue].msgs {
		if !m.deleted {
			out = append(out, *m)
		}
	}
	return out
}

func (f *fakeSQS) visible(m fakeMsg) bool { return !m.visibleAt.After(time.Now()) }

func (f *fakeSQS) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeSQS) usedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.keyIDs...)
}

func (f *fakeSQS) serve(w http.ResponseWriter, r *http.Request) {
	op := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "AmazonSQS.")
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	f.mu.Lock()
	f.calls = append(f.calls, op)
	if m := credentialRe.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		f.keyIDs = append(f.keyIDs, m[1])
	}
	f.mu.Unlock()

	var out map[string]any
	switch op {
	case "GetQueueUrl":
		out = f.getQueueURL(w, in)
	case "SendMessage":
		out = f.sendMessage(w, in)
	case "ReceiveMessage":
		out = f.receiveMessage(in)
	case "DeleteMessage":
		out = f.deleteMessage(in)
	case "ChangeMessageVisibility":
		out = f.changeVisibility(in)
	default:
		fail(w, "UnknownOperation", op)
		return
	}
	if out == nil {
		return // an error was written
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	_ = json.NewEncoder(w).Encode(out)
}

func fail(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": "com.amazonaws.sqs#" + code, "message": msg})
}

// queueOf finds the queue a request names by its URL.
func (f *fakeSQS) queueOf(url any) (string, *fakeQueue) {
	s, _ := url.(string)
	name := s[strings.LastIndex(s, "/")+1:]
	return name, f.queues[name]
}

func (f *fakeSQS) getQueueURL(w http.ResponseWriter, in map[string]any) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, _ := in["QueueName"].(string)
	if f.queues[name] == nil {
		fail(w, "QueueDoesNotExist", "no queue "+name)
		return nil
	}
	return map[string]any{"QueueUrl": f.queueURL(name)}
}

func (f *fakeSQS) sendMessage(w http.ResponseWriter, in map[string]any) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, q := f.queueOf(in["QueueUrl"])
	if q == nil {
		fail(w, "QueueDoesNotExist", "no queue "+name)
		return nil
	}
	body, _ := in["MessageBody"].(string)
	m := f.enqueue(name, body)
	m.group, _ = in["MessageGroupId"].(string)
	m.dedup, _ = in["MessageDeduplicationId"].(string)
	m.delay, _ = in["DelaySeconds"].(float64)
	if attrs, ok := in["MessageAttributes"].(map[string]any); ok {
		for k, v := range attrs {
			m.attrs[k], _ = v.(map[string]any)["StringValue"].(string)
		}
	}
	sum := md5.Sum([]byte(body)) //nolint:gosec // see the import
	return map[string]any{"MessageId": m.id, "MD5OfMessageBody": hex.EncodeToString(sum[:])}
}

func (f *fakeSQS) receiveMessage(in map[string]any) map[string]any {
	deadline := time.Now().Add(40 * time.Millisecond)
	for {
		if msgs := f.takeVisible(in); len(msgs) > 0 || time.Now().After(deadline) {
			return map[string]any{"Messages": msgs}
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (f *fakeSQS) takeVisible(in map[string]any) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, q := f.queueOf(in["QueueUrl"])
	limit := 10
	if n, ok := in["MaxNumberOfMessages"].(float64); ok {
		limit = int(n)
	}
	hide := 30 * time.Second
	if v, ok := in["VisibilityTimeout"].(float64); ok {
		hide = time.Duration(v) * time.Second
	}
	var out []map[string]any
	for _, m := range q.msgs {
		if m.deleted || !f.visible(*m) || len(out) == limit {
			continue
		}
		m.receives++
		m.handle = fmt.Sprintf("%s-handle-%d", m.id, m.receives)
		m.visibleAt = time.Now().Add(hide)
		sum := md5.Sum([]byte(m.body)) //nolint:gosec // see the import
		msg := map[string]any{"MessageId": m.id, "ReceiptHandle": m.handle, "Body": m.body, "MD5OfBody": hex.EncodeToString(sum[:]),
			"Attributes": map[string]string{"ApproximateReceiveCount": fmt.Sprint(m.receives), "MessageGroupId": m.group}}
		if len(m.attrs) > 0 {
			attrs := map[string]any{}
			for k, v := range m.attrs {
				attrs[k] = map[string]any{"DataType": "String", "StringValue": v}
			}
			msg["MessageAttributes"] = attrs
		}
		out = append(out, msg)
	}
	return out
}

func (f *fakeSQS) byHandle(in map[string]any) *fakeMsg {
	_, q := f.queueOf(in["QueueUrl"])
	handle, _ := in["ReceiptHandle"].(string)
	for _, m := range q.msgs {
		if m.handle == handle {
			return m
		}
	}
	return nil
}

func (f *fakeSQS) deleteMessage(in map[string]any) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m := f.byHandle(in); m != nil {
		m.deleted = true
	}
	return map[string]any{}
}

func (f *fakeSQS) changeVisibility(in map[string]any) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m := f.byHandle(in); m != nil {
		secs, _ := in["VisibilityTimeout"].(float64)
		m.visibleAt = time.Now().Add(time.Duration(secs) * time.Second)
	}
	return map[string]any{}
}
