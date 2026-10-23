// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// Section is a part of the request the left panel edits.
type Section string

// Sections, in the order the section dropdown lists them.
const (
	SectionParams   Section = "params"
	SectionHeaders  Section = "headers"
	SectionBody     Section = "body"
	SectionAuth     Section = "auth"
	SectionVars     Section = "variables"
	SectionSettings Section = "settings"
	SectionExtract  Section = "extract"
	SectionAssert   Section = "assert"
	SectionHooks    Section = "hooks"
	SectionDocs     Section = "docs"
	SectionSchema   Section = "schema"
)

// SectionAuto is the setting's "let the request decide": InitialSection
// picks the panel.
const SectionAuto Section = "auto"

// Configured reports whether the selected version has something in
// section s (the green dot). Hooks count when any run around the version.
func (r *Request) Configured(s Section) bool {
	v := r.Current()
	if v.Async != nil && asyncConfigured(*v.Async, s) {
		return true
	}
	if v.GRPC != nil && grpcConfigured(*v.GRPC, s) {
		return true
	}
	switch s {
	case SectionParams:
		return len(v.Query) > 0
	case SectionHeaders:
		if v.Async != nil && !AsyncHeadersAreConnection(v.Async.Protocol) {
			return v.Async.Send != nil && len(v.Async.Send.Headers) > 0
		}
		return slices.ContainsFunc(v.Headers, func(kv model.KeyValue) bool { return !strings.EqualFold(kv.Key, authHeader) })
	case SectionBody:
		return bodyHasContent(v.Body)
	case SectionAuth:
		kind, _ := r.Auth()
		return kind != AuthNone
	case SectionVars:
		return len(v.Variables) > 0
	case SectionDocs:
		return strings.TrimSpace(v.Docs) != ""
	case SectionSettings:
		return len(v.Settings) > 0
	case SectionExtract:
		return len(v.Extract) > 0
	case SectionAssert:
		return len(v.Assert) > 0
	case SectionHooks:
		return len(r.HookChain()) > 0
	}
	return false
}

// Tone is the colour class of a response status (the UI maps tones to
// colours; they are separate from the palette).
type Tone int

// Tones.
const (
	ToneNone        Tone = iota
	TonePending          // sent, no response yet: yellow
	ToneSuccess          // 2xx: green
	ToneRedirect         // 3xx: aqua blue
	ToneClientError      // 4xx: orange
	ToneServerError      // 5xx, or no response at all: brick red
	ToneCancelled        // gray
)

// ToneOfStatus is the tone of an HTTP status.
func ToneOfStatus(status int) Tone {
	switch {
	case status >= 200 && status < 300:
		return ToneSuccess
	case status >= 300 && status < 400:
		return ToneRedirect
	case status >= 400 && status < 500:
		return ToneClientError
	}
	return ToneServerError
}
