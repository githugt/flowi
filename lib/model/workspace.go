// SPDX-License-Identifier: Apache-2.0

package model

// Mount is a collection loaded into a workspace from a directory.
type Mount struct {
	ID   ID
	Path string // as the user gave it; may start with "~"
}

// Schedule runs a flow on a cron schedule while `flowi serve` is running.
// Schedules are personal, so they live in workspace.yaml, not in a
// collection.
type Schedule struct {
	Name       string // unique within the workspace; names the schedule in logs and commands
	Flow       string // a flow path or id
	Collection string // optional: a collection name or id when Flow is ambiguous
	Cron       string // five fields, or @hourly, @daily, …
	Env        string // environment name; "" uses defaults.environment
	Inputs     []KeyValue
	TZ         string // IANA zone name; "" is the machine's zone
	Disabled   bool
	// Watch makes each tick a watch pass: the flow runs and what its calls answered is compared with the last pass,
	// and a changed contract ends the run as "changed". The flow must be read-only.
	Watch bool
}

// Workspace is the content of workspace.yaml.
type Workspace struct {
	Meta
	Collections []Mount
	Schedules   []Schedule
	Settings    Settings
	File        string
}
