// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func TestFormatsFor_should_offer_the_tool_of_the_request_kind(t *testing.T) {
	tests := []struct {
		name string
		r    engine.PreviewRequest
		want []Format
	}{
		{"http", engine.PreviewRequest{Method: "GET", URL: "https://a.test"}, Formats()},
		{"grpc", engine.PreviewRequest{Method: "GRPC", URL: "grpc://h:50051/p.S/M"}, []Format{Grpcurl}},
		{"mqtt", engine.PreviewRequest{Method: "MQTT", URL: "mqtt://h/t"}, []Format{Mosquitto}},
		{"kafka", engine.PreviewRequest{Method: "KAFKA", URL: "kafka://h:9092/t"}, []Format{Kcat}},
		{"nats", engine.PreviewRequest{Method: "NATS", URL: "nats://h/s"}, []Format{NatsCLI}},
		{"redis", engine.PreviewRequest{Method: "REDIS", URL: "redis://h/c"}, []Format{RedisCLI}},
		{"ws", engine.PreviewRequest{Method: "WS", URL: "ws://h/x"}, []Format{Websocat}},
		{"postgres", engine.PreviewRequest{Method: "SQL", URL: "postgres://u@h/db"}, []Format{Psql}},
		{"mysql", engine.PreviewRequest{Method: "SQL", URL: "mysql://u@h/db"}, []Format{MySQL}},
		{"sqlite file", engine.PreviewRequest{Method: "SQL", URL: "data/app.db"}, []Format{SQLite}},
		{"a csv file is queried with duckdb", engine.PreviewRequest{Method: "SQL", URL: "data/people.csv"}, []Format{DuckDB}},
		{"a folder of files has no tool", engine.PreviewRequest{Method: "SQL", URL: "data/"}, nil},
		{"sqs", engine.PreviewRequest{Method: "SQS", URL: "https://sqs.eu-west-1.amazonaws.com/1/q"}, []Format{AwsCLI}},
		{"sns", engine.PreviewRequest{Method: "SNS", URL: "sns://t"}, []Format{AwsCLI}},
		{"pubsub", engine.PreviewRequest{Method: "PUBSUB", URL: "pubsub://p/t"}, []Format{Gcloud}},
		{"amqp", engine.PreviewRequest{Method: "AMQP", URL: "amqp://h/q"}, []Format{AmqpTools}},
		{"sse", engine.PreviewRequest{Method: "SSE", URL: "https://h/events"}, []Format{Curl}},
		{"service bus", engine.PreviewRequest{Method: "BUS", URL: "sb://ns.servicebus.windows.net/q"}, []Format{Curl}},
		{"amqp 1.0 has no common tool", engine.PreviewRequest{Method: "AMQP1", URL: "amqp://h/q"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatsFor(tt.r)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGenerate_should_write_the_command_of_each_kind(t *testing.T) {
	tests := []struct {
		name string
		f    Format
		r    engine.PreviewRequest
		want string
	}{
		{"grpc plaintext with metadata and message", Grpcurl,
			engine.PreviewRequest{Method: "GRPC", URL: "grpc://localhost:50051/shop.Orders/Get", Headers: []engine.Header{{Name: "authorization", Value: "Bearer <secret>"}}, BodyKind: "text", Body: `{"id":"7"}`},
			"grpcurl \\\n  -plaintext \\\n  -H 'authorization: Bearer <secret>' \\\n  -d '{\"id\":\"7\"}' \\\n  'localhost:50051' \\\n  'shop.Orders/Get'"},
		{"grpc over tls", Grpcurl,
			engine.PreviewRequest{Method: "GRPC", URL: "grpcs://api.test:443/p.S/M"},
			"grpcurl \\\n  'api.test:443' \\\n  'p.S/M'"},
		{"mqtt publish with a login", Mosquitto,
			engine.PreviewRequest{Method: "MQTT", URL: "mqtt://bob:<secret>@broker:1884/orders/new", BodyKind: "text", Body: "it's here"},
			"mosquitto_pub \\\n  -h 'broker' \\\n  -p 1884 \\\n  -t 'orders/new' \\\n  -u 'bob' \\\n  -P '<secret>' \\\n  -m 'it'\\''s here'"},
		{"mqtt listen", Mosquitto,
			engine.PreviewRequest{Method: "MQTT", URL: "mqtt://broker/orders"},
			"mosquitto_sub \\\n  -h 'broker' \\\n  -p 1883 \\\n  -t 'orders' \\\n  -C 1"},
		{"kafka produce with a header", Kcat,
			engine.PreviewRequest{Method: "KAFKA", URL: "kafka://a:9092,b:9092/events", Headers: []engine.Header{{Name: "trace", Value: "1"}}, BodyKind: "text", Body: `{"a":1}`},
			"printf %s '{\"a\":1}' | kcat \\\n  -b 'a:9092,b:9092' \\\n  -t 'events' \\\n  -P \\\n  -H 'trace=1'"},
		{"kafka consume", Kcat,
			engine.PreviewRequest{Method: "KAFKA", URL: "kafka://a:9092/events"},
			"kcat \\\n  -b 'a:9092' \\\n  -t 'events' \\\n  -C \\\n  -c 1 \\\n  -e"},
		{"nats publish", NatsCLI,
			engine.PreviewRequest{Method: "NATS", URL: "nats://localhost:4222/orders", BodyKind: "text", Body: "hi"},
			"nats \\\n  --server 'nats://localhost:4222' \\\n  pub 'orders' \\\n  'hi'"},
		{"redis publish", RedisCLI,
			engine.PreviewRequest{Method: "REDIS", URL: "redis://cache:6379/alerts", BodyKind: "text", Body: "up"},
			"redis-cli -u 'redis://cache:6379' \\\n  PUBLISH 'alerts' 'up'"},
		{"websocket send", Websocat,
			engine.PreviewRequest{Method: "WS", URL: "ws://h/echo", BodyKind: "text", Body: "ping"},
			"printf %s 'ping' | websocat -n1 'ws://h/echo'"},
		{"psql", Psql,
			engine.PreviewRequest{Method: "SQL", URL: "postgres://app:<secret>@db:5432/shop", BodyKind: "text", Body: "SELECT 1"},
			"psql 'postgres://app:<secret>@db:5432/shop' \\\n  -c 'SELECT 1'"},
		{"mysql", MySQL,
			engine.PreviewRequest{Method: "SQL", URL: "mysql://app:<secret>@db/shop", BodyKind: "text", Body: "SELECT 1"},
			"mysql \\\n  -h 'db' \\\n  -P 3306 \\\n  -u 'app' \\\n  -p'<secret>' \\\n  'shop' \\\n  -e 'SELECT 1'"},
		{"sqlite", SQLite,
			engine.PreviewRequest{Method: "SQL", URL: "data/app.db", BodyKind: "text", Body: "SELECT 1"},
			"sqlite3 'data/app.db' \\\n  'SELECT 1'"},
		// Before: these kinds had no Code section.
		{"sse", Curl,
			engine.PreviewRequest{Method: "SSE", URL: "https://h/events", Headers: []engine.Header{{Name: "Authorization", Value: "Bearer <secret>"}}},
			"curl -N 'https://h/events' \\\n  -H 'Accept: text/event-stream' \\\n  -H 'Authorization: Bearer <secret>'"},
		{"service bus send", Curl,
			engine.PreviewRequest{Method: "BUS", URL: "sb://ns.servicebus.windows.net/orders", BodyKind: "text", Body: "hi"},
			"curl -X POST 'https://ns.servicebus.windows.net/orders/messages' \\\n  -H \"Authorization: Bearer $(az account get-access-token --resource https://servicebus.azure.net --query accessToken --output tsv)\" \\\n  --data-raw 'hi'"},
		{"sqs send", AwsCLI,
			engine.PreviewRequest{Method: "SQS", URL: "https://sqs.eu-west-1.amazonaws.com/1/orders", BodyKind: "text", Body: "hi"},
			"aws sqs send-message \\\n  --queue-url 'https://sqs.eu-west-1.amazonaws.com/1/orders' \\\n  --message-body 'hi'"},
		{"sqs receive by name", AwsCLI,
			engine.PreviewRequest{Method: "SQS", URL: "sqs://orders"},
			"aws sqs receive-message \\\n  --queue-url \"$(aws sqs get-queue-url --queue-name 'orders' --query QueueUrl --output text)\" \\\n  --max-number-of-messages 1 \\\n  --wait-time-seconds 10"},
		{"sns publish to an arn", AwsCLI,
			engine.PreviewRequest{Method: "SNS", URL: "sns://arn:aws:sns:eu-west-1:1:alerts", BodyKind: "text", Body: "up"},
			"aws sns publish \\\n  --topic-arn 'arn:aws:sns:eu-west-1:1:alerts' \\\n  --message 'up'"},
		{"pubsub publish", Gcloud,
			engine.PreviewRequest{Method: "PUBSUB", URL: "pubsub://projects/shop/topics/orders", BodyKind: "text", Body: "hi"},
			"gcloud pubsub topics publish 'orders' \\\n  --message 'hi' \\\n  --project 'shop'"},
		{"pubsub pull", Gcloud,
			engine.PreviewRequest{Method: "PUBSUB", URL: "pubsub://projects/shop/subscriptions/mine"},
			"gcloud pubsub subscriptions pull 'mine' \\\n  --limit 1 \\\n  --auto-ack \\\n  --project 'shop'"},
		{"amqp publish", AmqpTools,
			engine.PreviewRequest{Method: "AMQP", URL: "amqp://bob:<secret>@mq:5672/orders", BodyKind: "text", Body: "hi"},
			"amqp-publish --url='amqp://bob:<secret>@mq:5672' \\\n  --routing-key='orders' \\\n  --body='hi'"},
		{"amqp get", AmqpTools,
			engine.PreviewRequest{Method: "AMQP", URL: "amqp://mq/orders"},
			"amqp-get --url='amqp://mq' \\\n  --queue='orders'"},
		{"duckdb over a csv", DuckDB,
			engine.PreviewRequest{Method: "SQL", URL: "data/people.csv", BodyKind: "text", Body: "SELECT * FROM people"},
			"duckdb -c 'CREATE VIEW \"people\" AS SELECT * FROM read_csv('\\''data/people.csv'\\''); SELECT * FROM people'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Generate(tt.f, tt.r)
			if err != nil || got != tt.want {
				t.Fatalf("err %v\n got:\n%s\nwant:\n%s", err, got, tt.want)
			}
		})
	}
}

func TestGenerate_Given_a_tool_that_does_not_fit_the_request_Then_it_says_so(t *testing.T) {
	_, kafkaForHTTP := Generate(Kcat, engine.PreviewRequest{Method: "GET", URL: "https://a.test"})
	_, curlForGRPC := Generate(Curl, engine.PreviewRequest{Method: "GRPC", URL: "grpc://h/p.S/M"})

	if kafkaForHTTP == nil || !strings.Contains(kafkaForHTTP.Error(), "an HTTP request") {
		t.Fatalf("kcat for HTTP: %v", kafkaForHTTP)
	}
	if curlForGRPC != nil {
		t.Fatalf("an HTTP format is still rendered for any request, as before: %v", curlForGRPC)
	}
}

func TestLabel_should_name_the_other_tools(t *testing.T) {
	for _, f := range []Format{Grpcurl, Mosquitto, Kcat, NatsCLI, RedisCLI, Websocat, Psql, MySQL, SQLite, AwsCLI, Gcloud, AmqpTools, DuckDB} {
		if Label(f) == string(f) {
			t.Errorf("%s has no label", f)
		}
	}
}
