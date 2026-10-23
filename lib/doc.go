// SPDX-License-Identifier: Apache-2.0

// Package flowi is the core library of Flow-I, an open-source tool for
// composing API requests into flows.
//
// It owns everything that is not user interface: the collection and flow
// model, on-disk storage, validation, request execution, the flow engine
// and impact analysis. The Flow-I CLI and GUI are thin layers over this
// package, and other programs may embed it directly.
package flowi
