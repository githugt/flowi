// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "github.com/githugt/flowi/app/internal/vm"

// TypeChoice is one type of request the bar under the address offers, with its icon. The protocol, database or file
// format inside a type is chosen by the address row's own pill.
type TypeChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// typeLabel names a type for the menu and the notice.
func typeLabel(id string) string {
	switch id {
	case vm.TypeHTTP:
		return "HTTP"
	case vm.TypeGRPC:
		return "gRPC"
	case vm.TypeMessage:
		return "messaging"
	case vm.TypeDB:
		return "database"
	}
	return "data file"
}

// typeMenuLabel is typeLabel as the menu writes it.
func typeMenuLabel(id string) string {
	switch id {
	case vm.TypeMessage:
		return "Messaging"
	case vm.TypeDB:
		return "Database"
	case vm.TypeFile:
		return "Data file"
	}
	return typeLabel(id)
}

func typeChoices() []TypeChoice {
	ids := vm.TypeIDs()
	out := make([]TypeChoice, 0, len(ids))
	for _, id := range ids {
		out = append(out, TypeChoice{ID: id, Label: typeMenuLabel(id), Icon: vm.TypeIcon(id)})
	}
	return out
}

// SwitchType makes the selected version another type of request, keeping what the types share and what can be
// carried across, and says in a notice what was left behind. It is an edit: Discard undoes it.
func (s *RequestService) SwitchType(node, id string) {
	s.with(node, func(o *openRequest) {
		was := vm.TypeOf(o.req.Current())
		lost, err := o.req.SwitchType(id)
		switch {
		case err != nil:
			s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
		case was != id:
			s.app.emit("notice", Notice{Level: "info", Text: vm.SwitchNote(typeLabel(id), lost)})
		}
	})
}

// SetName gives the request its own name, "" for none; saving keeps it.
func (s *RequestService) SetName(node, name string) {
	s.with(node, func(o *openRequest) { o.req.SetName(name) })
}
