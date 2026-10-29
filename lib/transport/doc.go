// SPDX-License-Identifier: Apache-2.0

// Package transport runs one async operation against a message server: it
// connects, subscribes, sends, collects the messages it was asked to wait
// for, and disconnects. Each protocol is a subpackage that implements
// Transport; transport/all wires them together.
//
// Connections are one-shot unless the transport also implements Opener: then
// a flow can keep a Session open across steps (WebSocket, MQTT 5, core NATS).
package transport
