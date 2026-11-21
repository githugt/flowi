// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
)

// OutputGitHub prints a diagnostic as a GitHub Actions workflow command, so the problem shows as an annotation on the
// file and the line in the pull request.
const OutputGitHub = "github"

// printGitHub writes one diagnostic as "::error file=…,line=…,col=…,title=CODE::message".
func printGitHub(w io.Writer, base string, d diag.Diagnostic) error {
	msg, hint := diag.English().Message(d)
	if hint != "" {
		msg += " (hint: " + hint + ")"
	}
	level := "error"
	if d.Severity == diag.SeverityWarning {
		level = "warning"
	}
	props := []string{}
	if d.File != "" {
		props = append(props, "file="+escapeWorkflowProperty(displayPath(base, d.File)))
		if d.Pos.Line > 0 {
			props = append(props, fmt.Sprintf("line=%d", d.Pos.Line))
			if d.Pos.Column > 0 {
				props = append(props, fmt.Sprintf("col=%d", d.Pos.Column))
			}
		}
	}
	props = append(props, "title="+escapeWorkflowProperty(string(d.Code)))
	_, err := fmt.Fprintf(w, "::%s %s::%s\n", level, strings.Join(props, ","), escapeWorkflowData(msg))
	return err
}

// escapeWorkflowData and escapeWorkflowProperty are the escapes GitHub reads in a workflow command.
func escapeWorkflowData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeWorkflowProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

// inGitHubActions says if the process runs in a GitHub Actions job.
func inGitHubActions(environ []string) bool {
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "GITHUB_ACTIONS="); ok {
			return v == "true"
		}
	}
	return false
}

// wantsAnnotations says if failures should also be printed as annotations: when asked for with --output github, and
// on its own in a GitHub Actions job.
func wantsAnnotations(output string, environ []string) bool {
	return output == OutputGitHub || (output != OutputJSON && inGitHubActions(environ))
}

// writeStepSummary appends a short Markdown report of the run to the file GitHub names in GITHUB_STEP_SUMMARY. It
// does nothing when the variable is not set.
func writeStepSummary(environ []string, res engine.RunResult, base string, failures diag.List) error {
	var path string
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "GITHUB_STEP_SUMMARY="); ok {
			path = v
		}
	}
	if path == "" {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### Flowi run: %s\n\n", res.Status)
	fmt.Fprintf(&b, "Run `%s`, seed `%d`", res.RunID, res.Seed)
	if res.ReportFile != "" {
		fmt.Fprintf(&b, ", report `%s`", displayPath(base, res.ReportFile))
	}
	b.WriteString("\n\n")
	if len(failures) > 0 {
		b.WriteString("| Code | Where | Problem |\n| --- | --- | --- |\n")
		for _, d := range failures {
			msg, _ := diag.English().Message(d)
			where := displayPath(base, d.File)
			if d.Path != "" {
				where += " " + d.Path
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", d.Code, tableCell(where), tableCell(msg))
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("writing the step summary: %w", err)
	}
	if _, err := io.WriteString(f, b.String()); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing the step summary: %w", err)
	}
	return f.Close()
}

func tableCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}
