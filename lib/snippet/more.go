// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"path"
	"strings"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// The tools of the kinds that had none: the clients a cloud's own command line offers, a RabbitMQ client, DuckDB for
// a data file; and cURL itself for the two that speak HTTP underneath (server-sent events, Service Bus).
//
// Before: SSE, SQS, SNS, Pub/Sub, Service Bus, AMQP and data file requests had no Code section at all, though each
// can be done by some command line.
const (
	AwsCLI    Format = "aws"
	Gcloud    Format = "gcloud"
	AmqpTools Format = "amqp-tools"
	DuckDB    Format = "duckdb"
)

func moreLabels() map[Format]string {
	return map[Format]string{
		AwsCLI: "AWS – aws cli", Gcloud: "Google Pub/Sub – gcloud", AmqpTools: "AMQP – amqp-publish / amqp-get", DuckDB: "Data file – duckdb",
	}
}

// moreTools are the formats of a message request of protocol kind that the file above adds, nil for the others.
func moreTools(kind string) []Format {
	switch kind {
	case model.ProtocolSSE, model.ProtocolServiceBus:
		return []Format{Curl}
	case model.ProtocolSQS, model.ProtocolSNS:
		return []Format{AwsCLI}
	case model.ProtocolPubSub:
		return []Format{Gcloud}
	case model.ProtocolAMQP:
		return []Format{AmqpTools}
	}
	return nil
}

// fileTool is DuckDB for a data file: the format is the file's extension.
func fileTool(url string) ([]Format, bool) {
	if _, ok := duckReader(url); ok {
		return []Format{DuckDB}, true
	}
	return nil, false
}

func duckReader(file string) (string, bool) {
	switch strings.ToLower(path.Ext(file)) {
	case ".csv":
		return "read_csv", true
	case ".xlsx":
		return "read_xlsx", true
	case ".parquet":
		return "read_parquet", true
	}
	return "", false
}

// curlForKind is the cURL command of the kinds that speak HTTP underneath, ok false for any other.
func curlForKind(r engine.PreviewRequest) (string, bool) {
	switch kindOf(r) {
	case model.ProtocolSSE:
		return curlSSE(r), true
	case model.ProtocolServiceBus:
		return curlServiceBus(r), true
	}
	return "", false
}

func curlSSE(r engine.PreviewRequest) string {
	args := []string{"curl -N " + shellQuote(r.URL), "-H 'Accept: text/event-stream'"}
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	return strings.Join(args, shellContinuation)
}

// curlServiceBus is the Service Bus REST call: a send posts the message to the queue or topic, a receive reads the
// head message under a lock (the message stays on the queue until the lock runs out).
func curlServiceBus(r engine.PreviewRequest) string {
	t := splitTarget(r.URL)
	base := "https://" + t.host + "/" + strings.TrimPrefix(t.path, "/")
	args := []string{}
	if hasBody(r) {
		args = append(args, "curl -X POST "+shellQuote(strings.TrimRight(base, "/")+"/messages"))
	} else {
		args = append(args, "curl -X POST "+shellQuote(strings.TrimRight(base, "/")+"/messages/head?timeout=10"))
	}
	args = append(args, `-H "Authorization: Bearer $(az account get-access-token --resource https://servicebus.azure.net --query accessToken --output tsv)"`)
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	if hasBody(r) {
		args = append(args, "--data-raw "+shellQuote(r.Body))
	}
	return strings.Join(args, shellContinuation)
}

// generateMore renders r in a format this file adds; ok is false when f is not one of them.
func generateMore(f Format, r engine.PreviewRequest) (text string, ok bool) {
	if _, known := moreLabels()[f]; !known {
		return "", false
	}
	t := splitTarget(r.URL)
	switch f {
	case AwsCLI:
		return awsCLI(r, t), true
	case Gcloud:
		return gcloud(r), true
	case AmqpTools:
		return amqpTools(r, t), true
	}
	return duckdb(r), true
}

// awsCLI is the aws command of an SQS queue or an SNS topic. The target is a queue URL or topic ARN as written, else a
// name that the command looks up.
func awsCLI(r engine.PreviewRequest, t target) string {
	if kindOf(r) == model.ProtocolSNS {
		arn := ""
		if i := strings.Index(r.URL, "arn:"); i >= 0 {
			arn = shellQuote(r.URL[i:])
		} else {
			arn = `"$(aws sns create-topic --name ` + shellQuote(lastSegment(r.URL)) + ` --query TopicArn --output text)"`
		}
		return strings.Join([]string{"aws sns publish", "--topic-arn " + arn, "--message " + shellQuote(r.Body)}, shellContinuation)
	}
	queue := ""
	if t.scheme == "http" || t.scheme == "https" {
		queue = shellQuote(r.URL)
	} else {
		queue = `"$(aws sqs get-queue-url --queue-name ` + shellQuote(lastSegment(r.URL)) + ` --query QueueUrl --output text)"`
	}
	if hasBody(r) {
		return strings.Join([]string{"aws sqs send-message", "--queue-url " + queue, "--message-body " + shellQuote(r.Body)}, shellContinuation)
	}
	return strings.Join([]string{"aws sqs receive-message", "--queue-url " + queue, "--max-number-of-messages 1", "--wait-time-seconds 10"}, shellContinuation)
}

// gcloud is the Pub/Sub command: publish to a topic, or pull one message from a subscription.
func gcloud(r engine.PreviewRequest) string {
	name := lastSegment(r.URL)
	var project []string
	if i := strings.Index(r.URL, "projects/"); i >= 0 {
		if p, _, _ := strings.Cut(r.URL[i+len("projects/"):], "/"); p != "" {
			project = []string{"--project " + shellQuote(p)}
		}
	}
	if hasBody(r) {
		return strings.Join(append([]string{"gcloud pubsub topics publish " + shellQuote(name), "--message " + shellQuote(r.Body)}, project...), shellContinuation)
	}
	return strings.Join(append([]string{"gcloud pubsub subscriptions pull " + shellQuote(name), "--limit 1", "--auto-ack"}, project...), shellContinuation)
}

// amqpTools is amqp-publish or amqp-get (the amqp-tools package). The last part of the path is the routing key or
// queue; what comes before it belongs to the server address (the virtual host).
func amqpTools(r engine.PreviewRequest, t target) string {
	server := t.scheme + "://"
	if t.userinfo != "" {
		server += t.userinfo + "@"
	}
	server += t.host
	rest := strings.Trim(t.path, "/")
	channel := lastSegment(rest)
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		server += "/" + rest[:i]
	}
	if hasBody(r) {
		return strings.Join([]string{"amqp-publish --url=" + shellQuote(server), "--routing-key=" + shellQuote(channel), "--body=" + shellQuote(r.Body)}, shellContinuation)
	}
	return strings.Join([]string{"amqp-get --url=" + shellQuote(server), "--queue=" + shellQuote(channel)}, shellContinuation)
}

// duckdb runs the query over the file: the file is a view named like it, as the query reads it.
func duckdb(r engine.PreviewRequest) string {
	reader, _ := duckReader(r.URL)
	stem := strings.TrimSuffix(path.Base(r.URL), path.Ext(r.URL))
	setup := ""
	if reader == "read_xlsx" {
		setup = "INSTALL excel; LOAD excel; "
	}
	sql := setup + `CREATE VIEW "` + stem + `" AS SELECT * FROM ` + reader + "(" + "'" + strings.ReplaceAll(r.URL, "'", "''") + "'" + "); " + r.Body
	return "duckdb -c " + shellQuote(sql)
}

// lastSegment is what follows the last "/" of s.
func lastSegment(s string) string {
	s = strings.TrimRight(s, "/")
	return s[strings.LastIndex(s, "/")+1:]
}
