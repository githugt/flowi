// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// shadowCollection is a collection with an environment that sets token and a flow whose request sends {{token}}.
func shadowCollection(t *testing.T, inputs []model.Input, header string) model.Collection {
	f := newFixture(t)
	c := f.collection("c")
	r := f.request(&c, "get", "v1")
	r.Versions[0].Headers = []model.KeyValue{{Key: "X-T", Value: header}}
	c.Requests[0] = r
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, File: c.Root + "/environments/prod.env.yaml", Vars: []model.EnvVar{{Name: "token", Value: "x"}}}}
	n := requestNode("call", r)
	n.Request.With = []model.KeyValue{{Key: "t", Value: "{{token}}"}}
	fl := f.flow(&c, "f", n)
	c.Flows[len(c.Flows)-1].Inputs = inputs
	_ = fl
	return c
}

func shadows(c model.Collection) diag.List {
	var out diag.List
	for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
		if d.Code == diag.CodeTemplateShadow {
			out = append(out, d)
		}
	}
	return out
}

func TestValidate_Given_a_flow_input_named_like_an_env_variable_When_used_bare_Then_it_is_flagged(t *testing.T) {
	// Before: the input hid the environment's token and nothing said so.
	c := shadowCollection(t, []model.Input{{Name: "token"}}, "x")

	got := shadows(c)

	if len(got) != 1 {
		t.Fatalf("want one shadow warning, got %v", got)
	}
	if msg := diag.Text(got[0]); !strings.Contains(msg, "{{token}}") || !strings.Contains(msg, "prod") || !strings.Contains(msg, "{{env.token}}") {
		t.Errorf("message does not say what is hidden and how to reach it: %q", msg)
	}
}

func TestValidate_Given_no_bare_use_or_no_clash_When_validated_Then_nothing_is_flagged(t *testing.T) {
	noClash := shadowCollection(t, []model.Input{{Name: "email"}}, "x")
	if got := shadows(noClash); len(got) != 0 {
		t.Errorf("an input that hides nothing was flagged: %v", got)
	}

	f := newFixture(t)
	c := f.collection("c")
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Vars: []model.EnvVar{{Name: "token", Value: "x"}}}}
	r := f.request(&c, "get", "v1")
	fl := f.flow(&c, "f", requestNode("call", r))
	c.Flows[len(c.Flows)-1].Inputs = []model.Input{{Name: "token"}}
	_ = fl
	if got := shadows(c); len(got) != 0 {
		t.Errorf("a name never written bare was flagged: %v", got)
	}
}
