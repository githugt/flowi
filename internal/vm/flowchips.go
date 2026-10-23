// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"

	"github.com/githugt/flowi-lib/model"
)

// ChipKind is what a chip in the flow header reports.
type ChipKind string

// The things a chip can report.
const (
	ChipError   ChipKind = "error"
	ChipNeeds   ChipKind = "needs"
	ChipWarning ChipKind = "warning"
	ChipDisk    ChipKind = "disk"
	ChipUnsaved ChipKind = "unsaved"
)

// Chip is one thing to tell about the flow. The view words it; the chip only
// carries the facts.
type Chip struct {
	Kind  ChipKind
	Text  string   // ChipError: the error
	Steps []string // ChipWarning: the steps only Flow-I can run, as "id (type)"; ChipNeeds: the needs on a later node, as "c needs b"
}

// FlowChips are the chips d needs now, most urgent first.
func FlowChips(d *FlowDoc) []Chip {
	return ChipsFor(d.Err(), d.ForwardNeeds(), d.FlowIOnlySteps(), d.ChangedOnDisk(), d.Dirty())
}

// ChipsFor is FlowChips over the document's state, so it needs no document.
func ChipsFor(err string, forwardNeeds, flowIOnly []string, changedOnDisk, dirty bool) []Chip {
	var chips []Chip
	if err != "" {
		chips = append(chips, Chip{Kind: ChipError, Text: err})
	}
	if len(forwardNeeds) > 0 {
		chips = append(chips, Chip{Kind: ChipNeeds, Steps: forwardNeeds})
	}
	if len(flowIOnly) > 0 {
		chips = append(chips, Chip{Kind: ChipWarning, Steps: flowIOnly})
	}
	if changedOnDisk {
		chips = append(chips, Chip{Kind: ChipDisk})
	}
	if dirty {
		chips = append(chips, Chip{Kind: ChipUnsaved})
	}
	return chips
}

// NodeSummary is the one line a node card shows under its name: the request
// or flow it calls, else its first parameter, else its type.
func NodeSummary(n model.Node) string {
	switch {
	case n.Request != nil:
		if n.Request.Version != "" {
			return n.Request.Request.Path + " v" + n.Request.Version
		}
		return n.Request.Request.Path
	case n.Subflow != nil:
		return "subflow " + n.Subflow.Flow.Path
	case len(n.Params) > 0:
		return fmt.Sprintf("%s: %v", n.Params[0].Key, n.Params[0].Value)
	}
	return string(n.Type)
}

// ParamText is the text of the node's parameter key, or "" when it has none.
func ParamText(n model.Node, key string) string {
	for _, p := range n.Params {
		if p.Key == key && p.Value != nil {
			return fmt.Sprint(p.Value)
		}
	}
	return ""
}
