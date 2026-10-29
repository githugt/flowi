// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/template"
)

// ExprContext says where an expression is written, which decides the names it
// can read.
type ExprContext string

// Where expressions are written.
const (
	ExprFlowTemplate   ExprContext = "flow.template" // {{…}} in a step's fields
	ExprFlowWhen       ExprContext = "flow.when"     // a step's "run when"
	ExprFlowCheck      ExprContext = "flow.check"    // assert, condition and run-till checks
	ExprRequestFields  ExprContext = "request.fields"
	ExprRequestCheck   ExprContext = "request.check"   // a request's assert and extract
	ExprMessageCheck   ExprContext = "message.check"   // a message request's assert and extract
	ExprMessageFilter  ExprContext = "message.filter"  // the "only if" of Receive
	ExprFlowTransform  ExprContext = "flow.transform"  // transform and calc values
	ExprFlowCollection ExprContext = "flow.collection" // batch "over"
)

// HelpItem is one name, function or operator with what it is and how to write it.
type HelpItem struct {
	Name    string
	What    string
	Example string
}

// ExprHelp is what a pop-up shows for one kind of expression.
type ExprHelp struct {
	Title    string
	Intro    string
	Names    []HelpItem // what can be read here
	Tools    []HelpItem // operators and functions
	Examples []string
}

func helpNodes() HelpItem {
	return HelpItem{"nodes.<step>.<output>", "What an earlier step produced. A request step has status, headers and body; other steps list their outputs in the Data tab after a run.", "{{nodes.login.body.token}}"}
}
func helpVars() HelpItem {
	return HelpItem{"vars.<name>", "A variable a step stored (extract, set, transform, calc, generate). Writing just {{name}} works too.", "{{vars.token}}"}
}
func helpInputs() HelpItem {
	return HelpItem{"inputs.<name>", "A value the flow was started with.", "{{inputs.email}}"}
}
func helpEnv() HelpItem {
	return HelpItem{"env.<name>", "A value of the selected environment.", "{{env.base_url}}"}
}
func helpSecret() HelpItem {
	return HelpItem{"secret.<name>", "A secret from the vault. A value built from it is masked in logs and reports.", "{{secret.password}}"}
}
func helpItem() HelpItem {
	return HelpItem{"item, index", "Inside a batch or run-till body: the current element and its position, from 0.", "{{item.id}}"}
}
func helpRun() HelpItem {
	return HelpItem{"run.id, run.iteration", "The id of this run, and which round of a repeating run it is.", "{{run.id}}"}
}
func helpGenName() HelpItem {
	return HelpItem{"$generator", "A fresh value each time. The run's seed makes them repeatable. Type $ for the list.", "{{$uuid-v4}}  {{$int(1,100)}}"}
}

func commonTools() []HelpItem {
	return []HelpItem{
		{"a ?? b", "b when a is missing or null; without it a missing value is an error.", `{{vars.nick ?? "friend"}}`},
		{"==  !=  <  >  <=  >=", "Compare. Durations can be written as 2s or 800ms.", "nodes.login.status == 200"},
		{"and  or  not  ( )", "Combine tests.", "status == 200 and body.error == empty"},
		{"x in [a, b]", "Is it one of several values.", `status in [200, 201]`},
		{"empty", "Matches null, an empty text, list or map.", "vars.token != empty"},
		{"len(x)  count(list, test)", "How many items or characters.", "len(nodes.list.body.items) > 0"},
		{"map  filter  any  all", "Work on lists. In the test, . is the current item.", "map(nodes.list.body.items, .id)"},
		{"upper  lower  trim  split  join  replace", "Work on text.", `upper(vars.code)`},
		{"a contains b   hasPrefix(a, b)   hasSuffix(a, b)", "Look inside text.", `headers["content-type"] contains "json"`},
		{"int  float  string  toJSON  fromJSON", "Convert values.", "int(vars.page) + 1"},
	}
}

// ExpressionHelp is the help for expressions written in context. An unknown
// context gets the help for step fields.
func ExpressionHelp(ctx ExprContext) ExprHelp {
	h, ok := helpByContext()[ctx]
	if !ok {
		h = helpByContext()[ExprFlowTemplate]
	}
	h.Tools = slices.Concat(h.Tools, commonTools())
	return h
}

func helpByContext() map[ExprContext]ExprHelp {
	flowNames := []HelpItem{helpNodes(), helpVars(), helpInputs(), helpEnv(), helpSecret(), helpItem(), helpRun()}
	return map[ExprContext]ExprHelp{
		ExprFlowTemplate: {
			Title:    "Values you can use in a step's fields",
			Intro:    "Put an expression between {{ and }} anywhere a field takes text. Everything outside the braces stays as it is.",
			Names:    append(slices.Clone(flowNames), helpGenName(), HelpItem{"request.*, response.*", "In a pre-hook: the request about to be sent. In a post-hook: the response (status, headers, body, time).", "{{response.body.id}}"}),
			Examples: []string{`Bearer {{vars.token}}`, `{{env.base_url}}/users/{{nodes.create.body.id}}`},
		},
		ExprFlowWhen: {
			Title:    "Run this step only when…",
			Intro:    "A test written without braces. When it is false the step is skipped and the flow goes on. To branch into different steps, use a condition step.",
			Names:    flowNames,
			Examples: []string{"nodes.login.status == 200", `inputs.mode == "full"`, "vars.count > 3 and vars.retry != empty"},
		},
		ExprFlowCheck: {
			Title:    "Values you can test",
			Intro:    "A test written without braces; it must be true or false.",
			Names:    flowNames,
			Examples: []string{"nodes.login.status == 200", "len(nodes.list.body.items) > 0", `nodes.job.body.state == "done"`},
		},
		ExprFlowTransform: {
			Title:    "Values you can reshape",
			Intro:    "Write one {{ }} expression. The result becomes the step's value (and the variable named in 'into').",
			Names:    flowNames,
			Examples: []string{"{{map(nodes.list.body.items, .id)}}", "{{filter(nodes.list.body.items, .active)}}", "{{vars.price * vars.count}}"},
		},
		ExprFlowCollection: {
			Title:    "The list to repeat over",
			Intro:    "A {{ }} expression that gives a list. The body runs once per item, with item and index.",
			Names:    flowNames,
			Examples: []string{"{{nodes.list.body.items}}", "{{split(inputs.ids, \",\")}}"},
		},
		ExprRequestFields: {
			Title: "Values you can use in a request",
			Intro: "Put an expression between {{ and }} in the URL, a header, a query value or the body.",
			Names: []HelpItem{
				helpEnv(), helpSecret(),
				{"var.<name>", "A variable of this request version; an environment value of the same name wins.", "{{var.page}}"},
				{"with.<name>", "A value the flow step passes in (its 'with' list).", "{{with.email}}"},
				helpVars(), helpInputs(), helpNodes(), helpGenName(),
			},
			Examples: []string{"{{env.base_url}}/v1/users?page={{var.page}}", `{"email": "{{$email}}", "password": "{{secret.password}}"}`},
		},
		ExprRequestCheck: {
			Title: "What you can check or read",
			Intro: "Assert checks are tests written without braces. Extract reads a value from the response into a variable.",
			Names: []HelpItem{
				{"status", "The HTTP status.", "status == 200"},
				{"body", "The response body; a JSON body is already parsed.", "body.data.id != empty"},
				{"headers", "Response headers; names are in lower case.", `headers["content-type"] contains "json"`},
				{"response.*", "The same as a whole: response.status, response.headers, response.body, response.time.", "response.time < 800ms"},
				helpEnv(), helpVars(), {"var.<name>", "A variable of this request version.", "var.page"},
			},
			Examples: []string{"status == 200", "body.items[0].id != empty", `headers["x-total"] == "10"`},
		},
		ExprMessageCheck: {
			Title: "What you can check or read in a message request",
			Intro: "Assert checks are tests written without braces. Hooks, extract and assert see the messages of the run once it has finished.",
			Names: []HelpItem{
				{"messages", "The messages received, as a list. Each has body, headers and key.", "messages[0].body.id"},
				{"all_messages", "Sent and received messages together.", "len(all_messages) == 2"},
				{"body, headers", "The first received message, or the sent one when nothing arrived.", "body.state == \"ok\""},
				{"received", "How many messages were received.", "received == 3"},
				{"outcome", "How the run ended: ok, timeout or error.", `outcome == "ok"`},
				helpEnv(), helpVars(),
			},
			Examples: []string{"received >= 1", `messages[0].body.event == "created"`, `outcome != "timeout"`},
		},
		ExprMessageFilter: {
			Title: "Which messages to keep",
			Intro: "One test written without braces, asked about each message on its own, in the same language as the checks of Assert. A message that lacks a field the test reads is skipped, not an error: a mistyped field name shows as a timeout with “received 0 of N”. The test cannot depend on several messages.",
			Names: []HelpItem{
				{"body.<field>", "A field of the message body.", `body.type == "order"`},
				{"headers.<name>", "A header or property of the message; names are in lower case.", `headers.event == "created"`},
			},
			Examples: []string{`body.type == "order" and body.total > 10`, `body.status in ["new", "paid"]`},
			Tools: []HelpItem{
				{"and, &&   or, ||   not, !", "Combine tests; parentheses group them.", `(body.a == 1 or body.b == 2) and not body.skip`},
				{"==  !=  <  <=  >  >=", "Compare a field with a value.", "body.total >= 10"},
				{"in [ … ]", "True when the field is one of the listed values.", `body.status in ["new", "paid"]`},
				{"received, outcome", "Not available here: they are known only once the run has ended.", ""},
			},
		},
	}
}

// GeneratorHelp lists the $generators with their description, for the pop-up's second tab.
func GeneratorHelp() []HelpItem {
	var out []HelpItem
	for _, g := range template.Generators() {
		out = append(out, HelpItem{Name: "$" + g.Name + g.Args, What: g.Description, Example: fmt.Sprintf("{{$%s%s}}", g.Name, strings.TrimSpace(g.Args))})
	}
	return out
}
