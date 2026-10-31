// SPDX-License-Identifier: Apache-2.0

// Package nodes implements every flow node type, one file each, and
// registers them with their parameter specs (Specs). The specs are the
// single source for the validator's parameter checks and for the generated
// node reference.
//
// A node only describes its work. Whatever needs the scheduler, the
// network, the clock or the workspace goes through State.Runtime, which
// the engine implements; this package never imports the engine.
package nodes
