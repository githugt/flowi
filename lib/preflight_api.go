// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// The kinds of Need.
const (
	NeedVariable    = "variable"     // {{env.name}} is read but the environment has no such variable
	NeedSecret      = "secret"       // {{secret.name}} is read but the environment declares no such secret
	NeedSecretValue = "secret-value" // the secret is declared but holds no value
)

// Need is something a run reads that the chosen environment does not provide.
type Need struct {
	Kind string
	Name string
}

// PreflightFlow lists what a run of flow t in environment env reads from the environment that it does not have: the
// {{env.x}} variables and {{secret.x}} secrets of the flow (draft, when given, instead of the saved one), of the
// flows it calls and of the requests it sends, with their hooks. It reads the text of those files, so a name that
// is only written in an expression that never runs still counts, and a variable a hook sets is not seen: treat
// the result as a warning, not as an error. No environment, no needs.
//
// Before: a missing variable or secret showed up in the middle of a run, after the earlier steps had already made
// their requests.
func (w *Workspace) PreflightFlow(t FlowTarget, env string, draft *model.Flow) ([]Need, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	if draft != nil {
		f = *draft
	}
	if env == "" {
		return nil, nil
	}
	all := w.Collections()
	skipped := filesNotRunByFlow(all, f)
	texts := []string{jsonText(f)}
	for _, coll := range all {
		for _, g := range coll.Flows {
			if !skipped[g.File] && g.File != f.File {
				texts = append(texts, jsonText(g))
			}
		}
		for _, r := range coll.Requests {
			if !skipped[r.File] {
				texts = append(texts, jsonText(r))
			}
		}
	}
	infos, err := w.Secrets()
	if err != nil {
		infos = nil // without the vault every declared secret counts as having no value: say so rather than stay quiet
	}
	return preflightNeeds(texts, w.Environments(), env, infos), nil
}

func jsonText(v any) string {
	b, _ := json.Marshal(v) // these model values always marshal; on failure the scan is just empty
	return string(b)
}

var (
	envReadPattern    = regexp.MustCompile(`(?:^|[^.\w])env\.([A-Za-z_][A-Za-z0-9_]*)`)
	secretReadPattern = regexp.MustCompile(`(?:^|[^.\w])secret\.([A-Za-z_][A-Za-z0-9_]*)`)
)

// preflightNeeds is PreflightFlow's check on the files' text. A secret counts only for the environment itself, a
// plain variable also from the environments it extends (as the engine reads them).
func preflightNeeds(texts []string, envs []model.Environment, env string, infos []SecretInfo) []Need {
	chain, err := model.EnvironmentChain(envs, env)
	if err != nil || len(chain) == 0 {
		return nil // an unknown or looping environment is reported by the run itself
	}
	own := chain[len(chain)-1]
	plain, secrets := map[string]bool{}, map[string]bool{}
	for _, e := range chain {
		for _, v := range e.Vars {
			switch {
			case v.Secret && e.Name == own.Name:
				secrets[v.Name] = true
				delete(plain, v.Name)
			case !v.Secret:
				plain[v.Name] = true
				delete(secrets, v.Name)
			}
		}
	}
	hasValue := map[string]bool{}
	for _, i := range infos {
		if i.Environment == own.Name && (i.Set || i.Override) {
			hasValue[i.Name] = true
		}
	}
	text := strings.Join(texts, "\n")
	var out []Need
	for _, name := range namesIn(envReadPattern, text) {
		if !plain[name] && !secrets[name] {
			out = append(out, Need{NeedVariable, name})
		}
	}
	for _, name := range namesIn(secretReadPattern, text) {
		switch {
		case !secrets[name]:
			out = append(out, Need{NeedSecret, name})
		case !hasValue[name]:
			out = append(out, Need{NeedSecretValue, name})
		}
	}
	return out
}

func namesIn(re *regexp.Regexp, text string) []string {
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// PreflightAll is PreflightFlow for every flow of every mounted collection, as warnings (FLW-RUN-028) on the flow
// files: `flowi validate --env staging` uses it to say whether a teammate or a CI job can run the flows there.
func (w *Workspace) PreflightAll(env string) (diag.List, error) {
	var out diag.List
	for _, c := range w.Collections() {
		for _, f := range c.Flows {
			needs, err := w.PreflightFlow(FlowTarget{Collection: c.Manifest.ID, Flow: f.ID}, env, nil)
			if err != nil {
				return nil, err
			}
			for _, n := range needs {
				out = append(out, needDiagnostic(f, env, n))
			}
		}
	}
	return out, nil
}

func needDiagnostic(f model.Flow, env string, n Need) diag.Diagnostic {
	what, problem := "{{env."+n.Name+"}}", "does not define it"
	switch n.Kind {
	case NeedSecret:
		what, problem = "{{secret."+n.Name+"}}", "does not declare that secret"
	case NeedSecretValue:
		what, problem = "{{secret."+n.Name+"}}", "has no value for that secret"
	}
	name := f.Name
	if name == "" {
		name = f.Path
	}
	return diag.New(diag.CodeRunEnvMissing, diag.Location{File: f.File}, map[string]string{"flow": name, "what": what, "env": env, "problem": problem})
}
