// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"

	"github.com/githugt/flowi/internal/vm"
)

// HooksView is the editor of a request's own hooks: the pre and post steps
// are two flow documents, edited with the flow editor's commands.
type HooksView struct {
	Request    string   `json:"request"` // the request row it belongs to
	Pre        string   `json:"pre"`     // the flow row of the pre-hooks document
	Post       string   `json:"post"`
	OthersPre  int      `json:"othersPre"`  // hook steps that run but are not edited here
	OthersPost int      `json:"othersPost"` // likewise
	Dirty      bool     `json:"dirty"`
	Blueprints []Choice `json:"blueprints"` // hook blueprints that can be added
}

type openHooks struct {
	editor *vm.HookEditor
}

// HooksService edits the hooks a request declares itself.
type HooksService struct {
	app   *App
	hooks map[string]*openHooks // by request row; used on the actor only
}

// newHooksService builds the service for the hook editor.
func newHooksService(a *App) *HooksService {
	return &HooksService{app: a, hooks: map[string]*openHooks{}}
}

func hookRow(request string, p vm.HookPhase) string { return "hooks:" + request + ":" + string(p) }

// Open opens the hooks of an open request. Its two documents are then open
// in the flow service under the rows in the view.
func (s *HooksService) Open(request string) (HooksView, error) {
	var v HooksView
	var err error
	s.app.actor.Do(func() {
		if h, ok := s.hooks[request]; ok {
			v = s.view(request, h)
			return
		}
		o, ok := s.app.Requests().reqs[request]
		if !ok {
			err = fmt.Errorf("open the request first")
			return
		}
		h := &openHooks{editor: vm.NewHookEditor(s.app.backend, s.app.worker, o.req)}
		for _, p := range []vm.HookPhase{vm.PhasePre, vm.PhasePost} {
			row, doc := hookRow(request, p), h.editor.Doc(p)
			s.app.Flows().adopt(row, doc)
			doc.OnChange(func() { s.app.emit("hooks", s.view(request, h)) })
		}
		s.hooks[request] = h
		v = s.view(request, h)
	})
	return v, err
}

// Close closes the hook editor, dropping unsaved edits.
func (s *HooksService) Close(request string) {
	s.app.actor.Do(func() {
		if _, ok := s.hooks[request]; !ok {
			return
		}
		for _, p := range []vm.HookPhase{vm.PhasePre, vm.PhasePost} {
			delete(s.app.Flows().docs, hookRow(request, p))
		}
		delete(s.hooks, request)
	})
}

// Save writes the edited hooks into the request and saves it. It returns
// "" when saved, "drops-comments" when the file's comments would be lost
// (call SaveDroppingComments to go on), or the problem.
func (s *HooksService) Save(request string) string { return s.save(request, false) }

// SaveDroppingComments saves although the request file's comments are lost.
func (s *HooksService) SaveDroppingComments(request string) string { return s.save(request, true) }

func (s *HooksService) save(request string, drop bool) string {
	ch := make(chan error, 1)
	s.app.actor.Do(func() {
		h, ok := s.hooks[request]
		if !ok {
			ch <- errors.New("the hook editor is not open")
			return
		}
		h.editor.Save(drop, func(err error) { ch <- err })
	})
	err := <-ch
	switch {
	case err == nil:
		return ""
	case vm.IsDropsComments(err):
		return "drops-comments"
	}
	return vm.ErrorText(err)
}

// AddBlueprint appends a hook blueprint's steps to its phase.
func (s *HooksService) AddBlueprint(request, key string) {
	s.app.actor.Do(func() {
		h, ok := s.hooks[request]
		if !ok {
			return
		}
		for _, bp := range h.editor.Blueprints(s.app.workspace) {
			if blueprintKey(bp) == key {
				h.editor.InsertBlueprint(bp)
			}
		}
	})
}

func (s *HooksService) view(request string, h *openHooks) HooksView {
	v := HooksView{
		Request: request, Pre: hookRow(request, vm.PhasePre), Post: hookRow(request, vm.PhasePost),
		OthersPre: h.editor.Others(vm.PhasePre), OthersPost: h.editor.Others(vm.PhasePost), Dirty: h.editor.Dirty(), Blueprints: []Choice{},
	}
	for _, bp := range h.editor.Blueprints(s.app.workspace) {
		v.Blueprints = append(v.Blueprints, Choice{ID: blueprintKey(bp), Label: bp.Name + " (" + bp.Phase + ")", Path: bp.Path})
	}
	return v
}
