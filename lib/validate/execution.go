// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// branchPolicies are the on_branch_error values ("" means fail_fast).
func branchPolicies() []string { return []string{"", "fail_fast", "finish_running", "continue_all"} }

// checkExecution checks a flow's execution and run blocks (FLW-FLOW-001).
func checkExecution(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, f := range c.Flows {
			out = append(out, executionProblems(f)...)
		}
	}
	return out
}

func executionProblems(f model.Flow) diag.List {
	var out diag.List
	bad := func(path, field, detail string) {
		out = append(out, diag.New(diag.CodeFlowOption, diag.Location{File: f.File, Path: path}, map[string]string{"field": field, "detail": detail}))
	}
	ex := yamlpath.Key("$", "execution")
	if f.Execution.RateLimit != "" {
		if _, err := settings.ParseRate(f.Execution.RateLimit); err != nil {
			bad(yamlpath.Key(ex, "rate_limit"), "execution.rate_limit", err.Error())
		}
	}
	for _, h := range f.Execution.HostRateLimits {
		if _, err := settings.ParseRate(h.Value); err != nil {
			bad(yamlpath.Key(yamlpath.Key(ex, "rate_limit"), h.Key), "execution.rate_limit."+h.Key, err.Error())
		}
	}
	if !slices.Contains(branchPolicies(), f.Execution.OnBranchError) {
		bad(yamlpath.Key(ex, "on_branch_error"), "execution.on_branch_error", "must be fail_fast, finish_running or continue_all")
	}
	if run := f.Run; run != nil {
		at := yamlpath.Key("$", "run")
		if run.Until == "" {
			bad(at, "run.until", "run needs an until condition")
		}
		if run.MaxIterations < 0 {
			bad(yamlpath.Key(at, "max_iterations"), "run.max_iterations", "must be at least 1")
		}
	}
	return out
}
