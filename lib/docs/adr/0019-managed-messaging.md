# 0019: Redis and managed messaging services are async protocols; cloud sign-in is a binding mode

- **Status:** Accepted
- **Date:** 2026-10-16
- **Builds on:** 0018 (async requests)

## Context

ADR 0018 gave async requests six protocols, all brokers a developer runs or connects to with a
user name and password. Teams also use managed services (AWS SQS and SNS, Azure Service Bus,
Google Pub/Sub) and Redis. Those differ in two ways: they sign in with the provider's own
credential chain rather than a password, and several are queues with acknowledgement rather than
streams. The owner asked for them now, with the sign-in chosen by a drop-down in an Authentication
panel.

## Decision

- **Five more protocols behind the same `transport.Transport` interface**: `redis` (pub/sub, and
  streams with optional consumer groups), `sqs`, `sns` (publish only), `servicebus` (peek-lock),
  `pubsub`. The `async:` block, records, hooks, schemas, Arazzo steps and CLI output do not change.
- **Sign-in is a binding mode.** Cloud protocols take binding `auth`: `default` (the provider's own
  lookup: AWS environment, profile and roles; Azure `DefaultAzureCredential`; Google Application
  Default Credentials) or `explicit` (credential bindings from the vault). `default` is stored as
  no entry, and never reads the credential bindings. The file format gains no new key.
- **Credentials are named per protocol** (`model.AsyncCredentials`) and the GUI splits one binding
  list into an Authentication section (credentials and the mode) and a Binding section (the rest).
  The same table drives `secretBindingName`, so credentials are never written to the generated
  AsyncAPI description, and the recorded target hides a password written in a server URL
  (`model.RedactedTarget`; this also closed a leak for the older protocols).
- **Queue-like services share one pull shape** (`transport/internal/pull`): a goroutine pulls and
  holds each message's handle, the receive keeps some, and at the end of the run the kept ones are
  acknowledged and every other handle is given back (SQS visibility 0, Pub/Sub nack, Service Bus
  abandon). Skipped messages are held until the end, not returned at once: an immediate return
  hands them straight back to the same run. Pub/Sub uses unary Pull/Acknowledge because the
  streaming `Receive` of the client library sends acknowledgements in the background and loses them
  when the client is closed.
- **SNS only publishes.** A version that receives from SNS, and a Redis pub/sub request that both
  sends and receives (it cannot tell its own message from a reply), are `FLW-ASYNC-002`. Redis streams can do both.
- **Service Bus uses the Azure SDK**, not a generic AMQP 1.0 client. A generic AMQP 1.0 transport
  stays in the backlog.
- **Verbs**: the request shows `REDIS`, `SQS`, `SNS`, `BUS`, `PUBSUB` where an HTTP request shows its
  method (`model.ProtocolVerb`).

## Consequences

- Dependencies grow: `go-redis`, the AWS SDK v2 (config, credentials, sqs, sns), the Azure SDK
  (`azservicebus`, `azidentity`), the Google Cloud Pub/Sub v2 client. Each is behind its transport
  package; the binary is noticeably larger.
- Redis (miniredis), Pub/Sub (pstest), SQS and SNS (in-process fakes of the few calls used) are tested
  in the normal suite. Azure Service Bus has unit tests of its mapping and validation only; its
  emulator needs SQL Server and sits behind the compose profile `servicebus`, so that transport's
  send and receive paths have not been exercised by the automated suite.
- `just test-integration` also starts Redis, LocalStack (SQS and SNS) and the Pub/Sub emulator.
