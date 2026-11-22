// SPDX-License-Identifier: Apache-2.0

// Package asyncapi imports an AsyncAPI document (3.x, and 2.x as far as it
// maps) as async requests: one request per operation, with the protocol,
// server, channel and an example of the message it sends.
//
// Three extensions say what AsyncAPI has no field for: x-flowi-protocol on a server (amqp10, servicebus),
// x-flowi-scheme on a server (the URL scheme to write, e.g. http for a local SQS emulator) and x-flowi-binding on an
// operation (Flowi binding names such as mode, start, endpoint or reply_to; never credentials).
package asyncapi
