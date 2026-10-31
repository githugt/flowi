// SPDX-License-Identifier: Apache-2.0

package model

// ParamType is the kind of value a blueprint parameter takes.
type ParamType string

// Parameter types. A request-ref or flow-ref parameter fills the places in
// the blueprint's nodes that name a request or a subflow ({param: name});
// the other types become inputs of the flow made from the blueprint.
const (
	ParamString     ParamType = "string"
	ParamInt        ParamType = "int"
	ParamBool       ParamType = "bool"
	ParamRequestRef ParamType = "request-ref"
	ParamFlowRef    ParamType = "flow-ref"
)

// ParamTypes lists the parameter types in documentation order.
func ParamTypes() []ParamType {
	return []ParamType{ParamString, ParamInt, ParamBool, ParamRequestRef, ParamFlowRef}
}

// Param is one declared blueprint parameter. A parameter without a Default
// must be given a value when the blueprint is used.
type Param struct {
	Name        string
	Type        ParamType
	Description string
	Default     string
}

// Blueprint is a flow template with declared parameters (design §4.8). Body
// holds the flow's inputs, outputs, execution, run settings and nodes; its
// identity fields are unused. Using a blueprint makes an ordinary flow and
// leaves no link back. Path is relative to blueprints/ without the suffix.
type Blueprint struct {
	Meta
	Path string
	// Phase says which hooks of a request the blueprint's nodes can be: "pre"
	// or "post"; empty means post. It does not matter to a blueprint used to
	// make a flow.
	Phase  string
	Params []Param
	// Outcomes are the outcomes a request starts with when it is seeded from the blueprint (see Outcome): the
	// same list for the request's opening version, unless that already writes its own.
	Outcomes []Outcome
	Body     Flow
	File     string
}
