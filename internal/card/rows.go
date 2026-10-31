// SPDX-License-Identifier: AGPL-3.0-or-later

package card

import (
	"strconv"

	"github.com/githugt/flowi-lib/model"
)

// rowSpec says how one row is named, drawn and edited; its summary comes from
// a builder over the step and its facts, and is "" when the part is empty or
// at its default.
type rowSpec struct {
	key, icon, editor string
	summary           func(model.Node, Facts, kindRule) string
}

// rowSpecs is every row a request step can show.
func rowSpecs() map[string]rowSpec {
	return map[string]rowSpec{
		"inputs":     {key: "inputs", icon: "inputs", editor: "inputs", summary: inputsSummary},
		"send":       {key: "send", icon: "send", editor: "message", summary: sendSummary},
		"receive":    {key: "receive", icon: "receive", editor: "receive", summary: receiveSummary},
		"connection": {key: "connection", icon: "connection", editor: "connection", summary: connectionSummary},
		"hooks":      {key: "hooks", icon: "hooks", editor: "hooks", summary: hooksSummary},
		"outcomes":   {key: "outcomes", icon: "outcomes", editor: "outcomes", summary: outcomesSummary},
		"settings":   {key: "settings", icon: "settings", editor: "settings", summary: settingsSummary},
	}
}

// inputsSummary is the auth, the query or variables, the headers and the body
// kind: whichever of them the kind has and the step fills.
func inputsSummary(_ model.Node, f Facts, k kindRule) string {
	var parts []string
	for _, p := range k.inputs {
		switch p {
		case "auth":
			parts = append(parts, f.Auth)
		case "query":
			parts = append(parts, count(f.Query, "query", "query"))
		case "variables":
			parts = append(parts, count(f.Variables, "variable", "variables"))
		case "headers":
			parts = append(parts, count(f.Headers, "header", "headers"))
		case "body":
			parts = append(parts, f.Body)
		}
	}
	return join(parts...)
}

// sendSummary is the message: its body kind, what rides with it, and the
// kind's own extras (QoS, key, session…). A kind that keeps its auth here has
// it first.
func sendSummary(_ model.Node, f Facts, k kindRule) string {
	if !f.Sends && f.Kind != "grpc" {
		return ""
	}
	parts := []string{}
	if k.authInSend {
		parts = append(parts, f.Auth)
	}
	parts = append(parts, f.Body, count(f.Props, k.prop, k.props))
	return join(append(parts, f.SendExtras...)...)
}

// receiveSummary is how many messages are waited for, for how long, and the
// kind's own extras (group, ack mode).
func receiveSummary(_ model.Node, f Facts, _ kindRule) string {
	if !f.Receives {
		return ""
	}
	parts := []string{count(f.ReceiveCount, "message", "messages"), f.ReceiveWait}
	return join(append(parts, f.ReceiveExtras...)...)
}

// connectionSummary says whether the step opens a connection, uses one an
// earlier step opened, and closes it.
func connectionSummary(n model.Node, _ Facts, _ kindRule) string {
	c := n.Request
	if c == nil {
		return ""
	}
	var parts []string
	if c.Over != "" {
		use := "uses " + c.Over
		if c.Close {
			use += ", then closes it"
		}
		parts = append(parts, use)
	}
	if c.Open != "" {
		parts = append(parts, "opens "+c.Open)
	}
	return join(parts...)
}

func hooksSummary(_ model.Node, f Facts, _ kindRule) string {
	var parts []string
	if f.PreHooks > 0 {
		parts = append(parts, strconv.Itoa(f.PreHooks)+" before")
	}
	if f.PostHooks > 0 {
		parts = append(parts, strconv.Itoa(f.PostHooks)+" after")
	}
	return join(parts...)
}

// settingsSummary lists only what differs from the defaults.
func settingsSummary(_ model.Node, f Facts, _ kindRule) string {
	var parts []string
	if f.Retry > 0 {
		parts = append(parts, "retry "+strconv.Itoa(f.Retry))
	}
	if f.Timeout != "" {
		parts = append(parts, "timeout "+f.Timeout)
	}
	if f.Region != "" {
		parts = append(parts, "region "+f.Region)
	}
	return join(parts...)
}

// outcomesSummary names the outcomes the request step can end with, the enabled ones in order: success and
// failure for a request that writes none. It is always shown, since the arrows leaving the card follow them.
func outcomesSummary(_ model.Node, f Facts, _ kindRule) string {
	if len(f.Outcomes) == 0 {
		return join(model.OutcomeSuccess, model.OutcomeFailure)
	}
	var names []string
	for _, o := range f.Outcomes {
		if !o.Disabled {
			names = append(names, o.Name)
		}
	}
	return join(names...)
}
