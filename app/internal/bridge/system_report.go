// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "strings"

// maxReportText bounds what the window may put in the log in one report: a loop of failing renders must not fill it.
const maxReportText = 2000

// ReportFrontendError writes an error the window caught (an uncaught exception or a rejected promise nobody
// awaited) to the app's log, so a crash leaves evidence beyond the toast. kind is "error" or "rejection".
// The text is cut to a fixed length and its line breaks are kept out of the log line.
// Before: the window showed a toast and nothing was kept; a report of "it said something went wrong" had no trace.
func (s *SystemService) ReportFrontendError(kind, message, stack string) {
	s.app.logger.Warn("the window reported an error", "Event", "FrontendError", "Kind", reportKind(kind),
		"Message", reportText(message), "Stack", reportText(stack))
}

func reportKind(kind string) string {
	if kind == "rejection" {
		return kind
	}
	return "error"
}

func reportText(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " | ").Replace(strings.TrimSpace(s))
	if len(s) > maxReportText {
		s = strings.ToValidUTF8(s[:maxReportText], "") + "…"
	}
	return s
}
