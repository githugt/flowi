// SPDX-License-Identifier: Apache-2.0

// Package report holds the result of a run as a tree — nodes, their hooks
// and attempts, container lanes — and writes it as JSON, JUnit XML,
// Markdown or plain text. It is pure data and formatting: the engine
// builds the tree from its events; this package never imports it.
//
// Everything in a report is already masked by whoever built it.
package report
