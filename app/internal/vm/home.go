// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "github.com/githugt/flowi/lib/model"

// Setup is what the workspace holds, counted for the home page. The Scratch try-out space is not counted: it
// is not something the person has set up. A collection that has not been saved to a folder yet is.
type Setup struct {
	Collections  int
	Requests     int
	HTTP         int // requests by what they do: plain HTTP,
	Messaging    int // messaging and streaming (WebSocket, SSE, MQTT, Kafka…),
	GRPC         int
	Datasets     int // and queries of a data store
	Flows        int
	Environments int
	SecretVars   int // variables declared secret, across the environments
	Schedules    int // enabled ones
	Blueprints   int
	Overlays     int
	WithHooks    int // requests with at least one hook step
	TracingOn    bool
	// Suggestable is how many flows Flowi can propose from what the requests of SuggestIn return, SuggestIn being the
	// first collection that has any. NoteSuggestions fills them.
	Suggestable int
	SuggestIn   model.ID
}

// NoteSuggestions records the flows Flowi can propose: count is asked for each collection, skip leaves out the
// try-out space, and the first collection with any is the one the home page offers them in.
func (s *Setup) NoteSuggestions(cols []model.Collection, skip func(model.Collection) bool, count func(model.ID) int) {
	for _, c := range cols {
		if skip(c) {
			continue
		}
		if n := count(c.Manifest.ID); n > 0 {
			s.Suggestable, s.SuggestIn = n, c.Manifest.ID
			return
		}
	}
}

// SetupOf counts what collections, environments and schedules hold. skip names the collections to leave out.
func SetupOf(cols []model.Collection, skip func(model.Collection) bool, envs []model.Environment, schedules []model.Schedule) Setup {
	var s Setup
	for _, c := range cols {
		if skip(c) {
			continue
		}
		s.Collections++
		s.Flows += len(c.Flows)
		s.Blueprints += len(c.Blueprints)
		s.Overlays += len(c.Overlays)
		for _, r := range c.Requests {
			s.countRequest(r)
		}
	}
	s.Environments = len(envs)
	for _, e := range envs {
		for _, v := range e.Vars {
			if v.Secret {
				s.SecretVars++
			}
		}
	}
	for _, sc := range schedules {
		if !sc.Disabled {
			s.Schedules++
		}
	}
	return s
}

func (s *Setup) countRequest(r model.Request) {
	s.Requests++
	if len(r.Hooks.Pre)+len(r.Hooks.Post)+len(r.Hooks.Message) > 0 {
		s.WithHooks++
	}
	// A request is what its first version is: versions of one request do not change its kind.
	var v model.RequestVersion
	if len(r.Versions) > 0 {
		v = r.Versions[0]
	}
	switch {
	case v.GRPC != nil:
		s.GRPC++
	case v.Async != nil:
		s.Messaging++
	case v.Dataset != nil:
		s.Datasets++
	default:
		s.HTTP++
	}
}

// FeatureIdea is a feature the person has not used yet, with why it would help and where to start.
type FeatureIdea struct {
	ID    string
	Title string
	Why   string
	// Action is what the button does: "screen:<name>" opens that screen; "new-request", "new-flow",
	// "new-environment" and "open-request" are the window's own actions; "suggest-flows:<collection>" opens the review
	// of suggested flows in that collection.
	Action string
	Label  string // the button's text
}

// featureRule says when a feature is worth offering: it is unused and what it builds on exists.
type featureRule struct {
	idea    FeatureIdea
	applies func(Setup) bool
}

// featureRules is the order the cards appear in, most useful first. Each rule holds only while the feature
// is unused, so a card goes away by itself once the person has tried what it offers. It is a function, not a
// variable, so nothing global can be changed.
func featureRules() []featureRule {
	return []featureRule{
		{FeatureIdea{"suggested-flows", "Flows found in your requests", "A request that makes something (a POST that returns an id) and others that take it in their path are already a flow: review the ones Flowi found.", "suggest-flows", "Review the flows"},
			func(s Setup) bool { return s.Suggestable > 0 && s.Flows == 0 }},
		{FeatureIdea{"environments", "Keep settings per environment", "Put the address and keys of local, staging and production in an environment, and switch between them in one click.", "new-environment", "New environment"},
			func(s Setup) bool { return s.Collections > 0 && s.Environments == 0 }},
		{FeatureIdea{"flows", "Chain requests into a flow", "Log in, create something, check it: a flow runs several requests in order and passes values along.", "new-flow", "New flow"},
			func(s Setup) bool { return s.Requests >= 3 && s.Flows == 0 && s.Suggestable == 0 }},
		{FeatureIdea{"hooks", "Check responses automatically", "A hook runs after a request: capture a token for the next call, or fail the run when something is wrong.", "open-request", "Open a request"},
			func(s Setup) bool { return s.Requests > 0 && s.WithHooks == 0 }},
		{FeatureIdea{"schedules", "Run a flow on a schedule", "Run a flow every hour or every morning and find out when an API breaks before your users do.", "screen:schedules", "Schedules"},
			func(s Setup) bool { return s.Flows > 0 && s.Schedules == 0 }},
		{FeatureIdea{"secrets", "Keep tokens out of your files", "Mark a variable as secret and its value is kept in your system's keychain, never written to the collection.", "screen:secrets", "Secrets"},
			func(s Setup) bool { return s.Environments > 0 && s.SecretVars == 0 }},
		{FeatureIdea{"overlays", "Try a request with changes, without editing it", "An overlay lays changes (another user, a bad input) over a request and leaves the original alone.", "open-request", "Open a request"},
			func(s Setup) bool { return s.Requests >= 3 && s.Overlays == 0 }},
		{FeatureIdea{"blueprints", "Reuse a flow as a blueprint", "Turn a flow you like into a template with blanks, and start new flows from it.", "screen:blueprints", "Blueprints"},
			func(s Setup) bool { return s.Flows > 0 && s.Blueprints == 0 }},
		{FeatureIdea{"tracing", "See every run in your tracing tool", "Send a trace of each run to Jaeger, Grafana or Honeycomb and see where the time goes.", "screen:tracing", "Tracing"},
			func(s Setup) bool { return s.Requests+s.Flows > 0 && !s.TracingOn }},
		{FeatureIdea{"protocols", "Not only HTTP", "Send and receive over WebSocket, server-sent events, MQTT, Kafka and more, call gRPC, or query a SQL database.", "new-request", "New request"},
			func(s Setup) bool { return s.HTTP > 0 && s.Messaging+s.GRPC+s.Datasets == 0 }},
	}
}

// SuggestFeatures lists the features this workspace has not used yet, in the order worth trying them.
func SuggestFeatures(s Setup) []FeatureIdea {
	var out []FeatureIdea
	for _, r := range featureRules() {
		if !r.applies(s) {
			continue
		}
		idea := r.idea
		if idea.ID == "suggested-flows" {
			idea.Action += ":" + string(s.SuggestIn) // the card opens the review in the collection that has them
		}
		out = append(out, idea)
	}
	return out
}
