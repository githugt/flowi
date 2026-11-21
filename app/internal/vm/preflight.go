// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"

	flowi "github.com/githugt/flowi/lib"
)

// PreflightLines say, one line each, what a run needs that env does not have. They end up in a question before the run
// starts: the missing value is found before the first request, not halfway.
func PreflightLines(env string, needs []flowi.Need) []string {
	lines := make([]string, 0, len(needs))
	for _, n := range needs {
		switch n.Kind {
		case flowi.NeedVariable:
			lines = append(lines, fmt.Sprintf("The variable %s is read but %s does not define it.", n.Name, env))
		case flowi.NeedSecret:
			lines = append(lines, fmt.Sprintf("The secret %s is read but %s does not declare it.", n.Name, env))
		case flowi.NeedSecretValue:
			lines = append(lines, fmt.Sprintf("The secret %s has no value in %s.", n.Name, env))
		}
	}
	return lines
}

// WriteLines say, one line each, which steps of a flow change data: "login sends POST auth/login", with the subflow
// the step is in when it is not the flow's own. They end up in the question before a run in a protected environment.
func WriteLines(writes []flowi.FlowWrite) []string {
	lines := make([]string, 0, len(writes))
	for _, w := range writes {
		line := fmt.Sprintf("%s: %s (%s)", w.Step, w.What, w.Request)
		if w.Via != "" {
			line += " in " + w.Via
		}
		lines = append(lines, line)
	}
	return lines
}
