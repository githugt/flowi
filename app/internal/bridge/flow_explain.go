// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "github.com/githugt/flowi/lib/analysis"

// SentenceView is one line of a flow told in words.
type SentenceView struct {
	Indent int    `json:"indent"`
	Text   string `json:"text"`
	Node   string `json:"node"` // the step the line is about, "" for a line that only introduces a group
}

// ProblemView is something about the flow worth a look, said about one step.
type ProblemView struct {
	Node string `json:"node"`
	Text string `json:"text"`
}

// ExplainView is the flow in words, for the storyboard.
type ExplainView struct {
	Sentences []SentenceView `json:"sentences"`
	Problems  []ProblemView  `json:"problems"`
}

// Explain tells the open flow in words, edits not yet saved included, and lists what looks wrong with it.
func (s *FlowService) Explain(node string) ExplainView {
	out := ExplainView{Sentences: []SentenceView{}, Problems: []ProblemView{}}
	_ = s.edit(node, func(o *openFlow) error {
		e := analysis.Explain(o.doc.Draft())
		for _, sentence := range e.Sentences {
			out.Sentences = append(out.Sentences, SentenceView{Indent: sentence.Indent, Text: sentence.Text, Node: sentence.Node})
		}
		for _, p := range e.Problems {
			out.Problems = append(out.Problems, ProblemView{Node: p.Node, Text: p.Text})
		}
		return nil
	})
	return out
}
