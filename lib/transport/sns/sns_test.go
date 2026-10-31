// SPDX-License-Identifier: Apache-2.0

package sns_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/sns"
)

const secretKey = "wJalrXUtnFEMI-s3cret-key"

var credentialRe = regexp.MustCompile(`Credential=([^/]+)/`)

// published is what the fake saw in one Publish call.
type published struct {
	topicARN, message, subject, group, dedup string
	attrs                                    map[string]string
}

// fakeSNS is an in-process stand-in for the SNS calls the transport makes
// (the AWS query protocol: form fields in, XML out).
type fakeSNS struct {
	URL string

	mu         sync.Mutex
	topics     []string // ARNs, served by ListTopics two to a page
	publishes  []published
	calls      []string
	keyIDs     []string
	nextMsgNum int
}

func startFakeSNS(t *testing.T, topics ...string) *fakeSNS {
	t.Helper()
	f := &fakeSNS{topics: topics}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

func (f *fakeSNS) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	action := r.Form.Get("Action")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, action)
	if m := credentialRe.FindStringSubmatch(r.Header.Get("Authorization")); m != nil {
		f.keyIDs = append(f.keyIDs, m[1])
	}
	w.Header().Set("Content-Type", "text/xml")
	switch action {
	case "ListTopics":
		f.listTopics(w, r)
	case "Publish":
		f.publish(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<ErrorResponse><Error><Type>Sender</Type><Code>InvalidAction</Code><Message>%s</Message></Error><RequestId>r</RequestId></ErrorResponse>`, action)
	}
}

// listTopics serves the topics two to a page; the page token is the index.
func (f *fakeSNS) listTopics(w http.ResponseWriter, r *http.Request) {
	start := 0
	if tok := r.Form.Get("NextToken"); tok != "" {
		if _, err := fmt.Sscanf(tok, "%d", &start); err != nil {
			start = 0 // an unreadable token restarts the listing
		}
	}
	end := min(start+2, len(f.topics))
	var b strings.Builder
	b.WriteString(`<ListTopicsResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><ListTopicsResult><Topics>`)
	for _, arn := range f.topics[start:end] {
		fmt.Fprintf(&b, "<member><TopicArn>%s</TopicArn></member>", arn)
	}
	b.WriteString("</Topics>")
	if end < len(f.topics) {
		fmt.Fprintf(&b, "<NextToken>%d</NextToken>", end)
	}
	b.WriteString(`</ListTopicsResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></ListTopicsResponse>`)
	_, _ = w.Write([]byte(b.String()))
}

func (f *fakeSNS) publish(w http.ResponseWriter, r *http.Request) {
	p := published{topicARN: r.Form.Get("TopicArn"), message: r.Form.Get("Message"), subject: r.Form.Get("Subject"),
		group: r.Form.Get("MessageGroupId"), dedup: r.Form.Get("MessageDeduplicationId"), attrs: map[string]string{}}
	for i := 1; ; i++ {
		name := r.Form.Get(fmt.Sprintf("MessageAttributes.entry.%d.Name", i))
		if name == "" {
			break
		}
		p.attrs[name] = r.Form.Get(fmt.Sprintf("MessageAttributes.entry.%d.Value.StringValue", i))
	}
	f.publishes = append(f.publishes, p)
	f.nextMsgNum++
	fmt.Fprintf(w, `<PublishResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><PublishResult><MessageId>sns-%d</MessageId></PublishResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></PublishResponse>`, f.nextMsgNum)
}

func (f *fakeSNS) published() []published {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]published(nil), f.publishes...)
}

func (f *fakeSNS) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeSNS) usedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.keyIDs...)
}

func operation(f *fakeSNS, channel string, extra map[string]string) transport.Operation {
	binding := map[string]string{"auth": "explicit", "access_key_id": "AKIDEXPLICIT", "secret_access_key": secretKey, "region": "us-east-1"}
	for k, v := range extra {
		binding[k] = v
	}
	return transport.Operation{Server: f.URL, Channel: channel, Binding: binding,
		Send: &transport.Message{Body: []byte("payload"), Headers: []transport.Header{{Name: "type", Value: "order"}}}}
}

const arn = "arn:aws:sns:us-east-1:000000000000:orders"

func TestRun_should_publish_to_a_topic_arn_as_given(t *testing.T) {
	f := startFakeSNS(t, arn)
	op := operation(f, arn, map[string]string{"subject": "New order", "group_id": "g1", "dedup_id": "d1"})

	res, err := sns.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	got := f.published()
	if len(got) != 1 || got[0].topicARN != arn || got[0].message != "payload" || got[0].subject != "New order" ||
		got[0].group != "g1" || got[0].dedup != "d1" || got[0].attrs["type"] != "order" {
		t.Fatalf("published %+v", got)
	}
	if calls := f.callList(); len(calls) != 1 {
		t.Fatalf("calls %v, want only Publish for an ARN", calls)
	}
	if res.Sent == nil || res.Sent.Direction != "sent" {
		t.Fatalf("sent %+v", res.Sent)
	}
	var id string
	for _, h := range res.Sent.Headers {
		if h.Name == "message_id" {
			id = h.Value
		}
	}
	if id != "sns-1" {
		t.Fatalf("message_id %q, want sns-1", id)
	}
}

func TestRun_should_find_a_topic_by_name_across_pages(t *testing.T) {
	f := startFakeSNS(t,
		"arn:aws:sns:us-east-1:000000000000:alpha",
		"arn:aws:sns:us-east-1:000000000000:order-events", // a longer name that merely ends the same way must not match
		arn)
	op := operation(f, "orders", nil)

	_, err := sns.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	got := f.published()
	if len(got) != 1 || got[0].topicARN != arn {
		t.Fatalf("published %+v, want the topic %s", got, arn)
	}
	if calls := strings.Join(f.callList(), ","); calls != "ListTopics,ListTopics,Publish" {
		t.Fatalf("calls %s, want two ListTopics pages then Publish", calls)
	}
}

func TestRun_should_say_when_no_topic_has_the_name(t *testing.T) {
	f := startFakeSNS(t, arn)

	_, err := sns.New().Run(t.Context(), operation(f, "missing", nil))

	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err %v, want one naming the topic", err)
	}
}

func TestRun_should_refuse_to_receive(t *testing.T) {
	f := startFakeSNS(t, arn)
	op := operation(f, arn, nil)
	op.Receive = &transport.Receive{Count: 1}

	_, err := sns.New().Run(t.Context(), op)

	if err == nil || !strings.Contains(err.Error(), "publish") {
		t.Fatalf("err %v, want one saying SNS only publishes", err)
	}
	if len(f.callList()) != 0 {
		t.Fatalf("calls %v, want nothing sent before the refusal", f.callList())
	}
}

func TestRun_should_refuse_a_body_that_is_not_text(t *testing.T) {
	f := startFakeSNS(t, arn)
	op := operation(f, arn, nil)
	op.Send.Body = []byte{0xff, 0xfe}

	_, err := sns.New().Run(t.Context(), op)

	if err == nil {
		t.Fatal("want an error for a binary body")
	}
}

func TestRun_should_sign_with_the_explicit_credentials(t *testing.T) {
	f := startFakeSNS(t, arn)

	if _, err := sns.New().Run(t.Context(), operation(f, arn, nil)); err != nil {
		t.Fatal(err)
	}

	if keys := f.usedKeys(); len(keys) == 0 || keys[0] != "AKIDEXPLICIT" {
		t.Fatalf("requests were signed with %v, want AKIDEXPLICIT", keys)
	}
}

func TestRun_should_use_the_provider_login_and_ignore_binding_credentials_when_auth_is_default(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDFROMENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/none")
	f := startFakeSNS(t, arn)

	if _, err := sns.New().Run(t.Context(), operation(f, arn, map[string]string{"auth": "default"})); err != nil {
		t.Fatal(err)
	}

	if keys := f.usedKeys(); len(keys) == 0 || keys[0] != "AKIDFROMENV" {
		t.Fatalf("requests were signed with %v, want AKIDFROMENV", keys)
	}
}

func TestRun_should_name_the_missing_credential_when_auth_is_explicit(t *testing.T) {
	f := startFakeSNS(t, arn)

	_, err := sns.New().Run(t.Context(), operation(f, arn, map[string]string{"access_key_id": ""}))

	if err == nil || !strings.Contains(err.Error(), "access_key_id") {
		t.Fatalf("err %v, want one naming access_key_id", err)
	}
}

func TestRun_should_not_put_the_secret_key_in_an_error(t *testing.T) {
	op := operation(&fakeSNS{URL: "http://127.0.0.1:1"}, arn, nil)

	_, err := sns.New().Run(t.Context(), op)

	if err == nil || strings.Contains(err.Error(), secretKey) {
		t.Fatalf("err %v, want a failure that does not contain the secret key", err)
	}
}
