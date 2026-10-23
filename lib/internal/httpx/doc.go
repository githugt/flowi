// SPDX-License-Identifier: Apache-2.0

// Package httpx sends one Flow-I request: protocol selection (auto, 1.1,
// 2, h2c), proxy and custom CAs, redirect chain capture, timings,
// response decompression, request bodies (including file uploads that
// must stay inside the collection), large downloads streamed to disk,
// retries with backoff and jitter, and a cookie jar that can be saved.
//
// It knows nothing about templates or secrets: callers build the request
// and mask what they record.
package httpx
