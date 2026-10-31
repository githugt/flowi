// SPDX-License-Identifier: AGPL-3.0-or-later

package card

import (
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// kindRule is what one request kind's card has: which rows apply (in the order
// they are shown), which parts the Inputs row is made of, the words for what
// rides with a message, and the kind's action words.
type kindRule struct {
	rows       []string
	inputs     []string
	authInSend bool // the kind has no Inputs row: its auth sits in Send
	prop       string
	props      string
	sendWord   string // the badge when the step sends
	recvWord   string // the badge when it only receives
}

// kindRules is the table behind the design note's "Request steps, by kind".
// Rules that follow from it: a kind that cannot receive has no Receive row;
// one that cannot send (SSE) has no Send row; only the kinds that keep a
// connection open between steps (WebSocket, MQTT, NATS) have a Connection row.
func kindRules() map[string]kindRule {
	return map[string]kindRule{
		"http":       {rows: []string{"inputs", "hooks", "outcomes", "settings"}, inputs: []string{"auth", "query", "headers", "body"}, sendWord: "GET"},
		"graphql":    {rows: []string{"inputs", "hooks", "outcomes", "settings"}, inputs: []string{"auth", "variables", "headers"}, sendWord: "QUERY"},
		"grpc":       {rows: []string{"send", "hooks", "outcomes", "settings"}, authInSend: true, prop: "metadata", props: "metadata", sendWord: "UNARY"},
		"dataset":    {rows: []string{"send", "hooks", "outcomes", "settings"}, prop: "param", props: "params", sendWord: "SQL"},
		"ws":         {rows: []string{"inputs", "send", "receive", "connection", "hooks", "outcomes"}, inputs: []string{"auth", "headers"}, sendWord: "SEND", recvWord: "LISTEN"},
		"sse":        {rows: []string{"inputs", "receive", "hooks", "outcomes", "settings"}, inputs: []string{"auth", "query", "headers"}, recvWord: "LISTEN", sendWord: "LISTEN"},
		"mqtt":       {rows: []string{"send", "receive", "connection", "hooks", "outcomes", "settings"}, prop: "property", props: "properties", sendWord: "PUBLISH", recvWord: "SUBSCRIBE"},
		"nats":       {rows: []string{"send", "receive", "connection", "hooks", "outcomes"}, prop: "header", props: "headers", sendWord: "PUBLISH", recvWord: "SUBSCRIBE"},
		"kafka":      {rows: []string{"send", "receive", "hooks", "outcomes", "settings"}, prop: "header", props: "headers", sendWord: "PRODUCE", recvWord: "CONSUME"},
		"amqp":       {rows: []string{"send", "receive", "hooks", "outcomes", "settings"}, prop: "property", props: "properties", sendWord: "PUBLISH", recvWord: "CONSUME"},
		"amqp10":     {rows: []string{"send", "receive", "hooks", "outcomes", "settings"}, prop: "property", props: "properties", sendWord: "PUBLISH", recvWord: "CONSUME"},
		"redis":      {rows: []string{"send", "receive", "hooks", "outcomes"}, prop: "field", props: "fields", sendWord: "PUBLISH", recvWord: "READ"},
		"sns":        {rows: []string{"send", "hooks", "outcomes", "settings"}, prop: "attribute", props: "attributes", sendWord: "PUBLISH"},
		"sqs":        {rows: []string{"send", "receive", "hooks", "outcomes", "settings"}, prop: "attribute", props: "attributes", sendWord: "SEND", recvWord: "RECEIVE"},
		"pubsub":     {rows: []string{"send", "receive", "hooks", "outcomes"}, prop: "attribute", props: "attributes", sendWord: "PUBLISH", recvWord: "PULL"},
		"servicebus": {rows: []string{"send", "receive", "hooks", "outcomes", "settings"}, prop: "property", props: "properties", sendWord: "SEND", recvWord: "RECEIVE"},
	}
}

func summarizeRequest(n model.Node, f Facts) Card {
	rule, ok := kindRules()[f.Kind]
	if !ok {
		rule = kindRules()["http"]
	}
	c := Card{Badge: badgeOf(f, rule), Subline: f.Target, Ports: donePorts()}
	if len(f.Outcomes) > 0 {
		c.Ports = outcomePorts(f.Outcomes)
	}
	specs := rowSpecs()
	for _, key := range rule.rows {
		spec := specs[key]
		row := Row{Key: spec.key, Icon: spec.icon, Editor: spec.editor, Summary: spec.summary(n, f, rule)}
		if row.Summary == "" {
			c.More = append(c.More, row)
			continue
		}
		c.Rows = append(c.Rows, row)
	}
	return c
}

// badgeOf is the action word: the one the version chose, else the kind's word
// for what the step does.
func badgeOf(f Facts, rule kindRule) string {
	if f.Method != "" {
		return strings.ToUpper(f.Method)
	}
	if f.Receives && !f.Sends && rule.recvWord != "" {
		return rule.recvWord
	}
	return rule.sendWord
}
