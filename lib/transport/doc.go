// SPDX-License-Identifier: Apache-2.0

// Package transport runs one async operation against a message server: it
// connects, subscribes, sends, collects the messages it was asked to wait
// for, and disconnects. Each protocol is a subpackage that implements
// Transport; transport/all wires them together.
//
// Connections are one-shot: nothing is kept between two Run calls.
package transport
