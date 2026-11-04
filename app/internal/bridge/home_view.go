// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// IdeaView is a feature the person has not used yet.
type IdeaView struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Why    string `json:"why"`
	Action string `json:"action"`
	Label  string `json:"label"`
}

// HomeView is what the home page shows about the workspace: counts of what is set up, and the features that are
// not used yet.
type HomeView struct {
	Collections  int        `json:"collections"`
	Requests     int        `json:"requests"`
	HTTP         int        `json:"http"`
	Messaging    int        `json:"messaging"`
	GRPC         int        `json:"grpc"`
	Datasets     int        `json:"datasets"`
	Flows        int        `json:"flows"`
	Environments int        `json:"environments"`
	SecretVars   int        `json:"secretVars"`
	Schedules    int        `json:"schedules"`
	Blueprints   int        `json:"blueprints"`
	Overlays     int        `json:"overlays"`
	WithHooks    int        `json:"withHooks"`
	TracingOn    bool       `json:"tracingOn"`
	Ideas        []IdeaView `json:"ideas"`
}

// Home counts the workspace as it is now. The page asks again when the workspace changes.
func (s *WelcomeService) Home() HomeView {
	var v HomeView
	s.app.actor.Do(func() {
		a := s.app
		setup := vm.SetupOf(a.backend.Collections(), a.workspace.IsScratch, a.workspace.Environment(), a.backend.Schedules())
		setup.TracingOn = a.tracing.vm.View().Running
		setup.NoteSuggestions(a.backend.Collections(), a.workspace.IsScratch, func(id model.ID) int {
			list, _ := a.backend.SuggestFlows(id) // a collection that cannot be read has nothing to offer here
			return len(list)
		})
		v = homeViewOf(setup)
	})
	return v
}

func homeViewOf(s vm.Setup) HomeView {
	v := HomeView{
		Collections: s.Collections, Requests: s.Requests, HTTP: s.HTTP, Messaging: s.Messaging, GRPC: s.GRPC, Datasets: s.Datasets,
		Flows: s.Flows, Environments: s.Environments, SecretVars: s.SecretVars, Schedules: s.Schedules, Blueprints: s.Blueprints,
		Overlays: s.Overlays, WithHooks: s.WithHooks, TracingOn: s.TracingOn, Ideas: []IdeaView{},
	}
	for _, i := range vm.SuggestFeatures(s) {
		v.Ideas = append(v.Ideas, IdeaView{ID: i.ID, Title: i.Title, Why: i.Why, Action: i.Action, Label: i.Label})
	}
	return v
}
