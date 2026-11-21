// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// maxRetries is the most a step may be retried: more would hide a step that does not work.
const maxRetries = 10

// checkRetries checks retries and retry_wait: a count from 0 to 10 on a step that can be retried, and a wait that is
// not negative and means something only with retries.
func checkRetries(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, l := range sites.NodeLists(c) {
			for i, n := range l.Value {
				out = append(out, retryProblems(l, i, n)...)
			}
		}
	}
	return out
}

func retryProblems(l sites.NodeList, i int, n model.Node) diag.List {
	at := func(key string) diag.Location {
		return diag.Location{File: l.File, Path: yamlpath.Key(yamlpath.Index(l.Path, i), key)}
	}
	var out diag.List
	switch {
	case n.Retries < 0 || n.Retries > maxRetries:
		out = append(out, paramDiag(n, "retries", "must be a number from 0 to 10", at("retries"), diag.SeverityError))
	case n.Retries > 0 && !n.Type.CanRetry():
		out = append(out, paramDiag(n, "retries", "only a request, fetch, webhook or subflow step can be retried", at("retries"), diag.SeverityError))
	}
	switch {
	case n.RetryWait < 0:
		out = append(out, paramDiag(n, "retry_wait", "must not be negative", at("retry_wait"), diag.SeverityError))
	case n.RetryWait > 0 && n.Retries == 0:
		out = append(out, paramDiag(n, "retry_wait", "has no effect without retries", at("retry_wait"), diag.SeverityWarning))
	}
	return out
}
