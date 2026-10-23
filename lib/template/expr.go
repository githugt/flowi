// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
)

// Names injected into every evaluation. The "__" prefix keeps them out of
// the user's namespace and out of reference analysis.
const (
	refPrefix   = "__r"
	genFunc     = "__gen"
	isEmptyFunc = "__isEmpty"
	emptyIdent  = "empty"
	secretRoot  = "secret"
)

// expression is one compiled placeholder.
type expression struct {
	program *vm.Program
	chains  []chain
}

// chain is a variable reference with constant steps (a.b, a["b"], a[0]).
// Each chain is replaced in the compiled program by a synthetic variable
// (__r<i>) whose value is resolved before running, which is how undefined
// references are caught and secret reads are tracked.
type chain struct {
	path    []any // string or int steps; path[0] is the root name
	guarded bool  // under ?? or ?.: a miss is nil, not an error
}

func (c chain) root() string { return c.path[0].(string) }

func (c chain) dotted() string {
	var b strings.Builder
	for i, step := range c.path {
		switch s := step.(type) {
		case int:
			b.WriteString("[" + strconv.Itoa(s) + "]")
		case string:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(s)
		}
	}
	return b.String()
}

// UndefinedError reports a reference that does not resolve. Name is the
// full dotted path as written, e.g. "nodes.login.token".
type UndefinedError struct{ Name string }

func (e *UndefinedError) Error() string { return "undefined " + e.Name }

// UnknownGeneratorError reports a $generator that does not exist, or any
// generator use when the scope has none.
type UnknownGeneratorError struct{ Name string }

func (e *UnknownGeneratorError) Error() string { return "unknown generator $" + e.Name }

// Undefined returns the names of every undefined reference in err, which
// may join several (Render reports them all, not just the first).
func Undefined(err error) []string {
	var names []string
	var walk func(error)
	walk = func(e error) {
		// Walks the errors.Join tree by hand: errors.As would stop at the
		// first match, and every name is wanted.
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			for _, inner := range joined.Unwrap() {
				walk(inner)
			}
			return
		}
		var ue *UndefinedError
		if errors.As(e, &ue) {
			names = append(names, ue.Name)
		}
	}
	if err != nil {
		walk(err)
	}
	return names
}

// compile rewrites and compiles one expression.
func compile(src string) (*expression, error) {
	rewritten := rewrite(src)
	tree, err := parser.Parse(rewritten)
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	p := &refPatcher{lets: letNames(tree)}
	program, err := expr.Compile(rewritten, expr.Patch(p))
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	return &expression{program: program, chains: p.chains}, nil
}

// firstLine drops expr's source excerpt, which repeats the rewritten text
// rather than what the user wrote.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func letNames(tree *parser.Tree) map[string]bool {
	c := letCollector{names: map[string]bool{}}
	ast.Walk(&tree.Node, &c)
	return c.names
}

type letCollector struct{ names map[string]bool }

func (c *letCollector) Visit(n *ast.Node) {
	if v, ok := (*n).(*ast.VariableDeclaratorNode); ok {
		c.names[v.Name] = true
	}
}

// refPatcher replaces reference chains with synthetic variables and
// comparisons with empty with __isEmpty calls. ast.Walk is post-order, so
// an identifier is replaced first and each member access on it then
// extends the same chain.
type refPatcher struct {
	lets   map[string]bool
	chains []chain
}

func (p *refPatcher) Visit(n *ast.Node) {
	switch x := (*n).(type) {
	case *ast.IdentifierNode:
		p.visitIdentifier(n, x)
	case *ast.MemberNode:
		p.visitMember(n, x)
	case *ast.BinaryNode:
		p.visitBinary(n, x)
	}
}

func (p *refPatcher) visitIdentifier(n *ast.Node, x *ast.IdentifierNode) {
	name := x.Value
	if strings.HasPrefix(name, "__") || name == emptyIdent || p.lets[name] {
		return
	}
	if _, isBuiltin := builtin.Index[name]; isBuiltin {
		return
	}
	p.chains = append(p.chains, chain{path: []any{name}})
	ast.Patch(n, &ast.IdentifierNode{Value: refName(len(p.chains) - 1)})
}

func (p *refPatcher) visitMember(n *ast.Node, x *ast.MemberNode) {
	idx, isRef := refOf(x.Node)
	step, constant := constantStep(x.Property)
	if !isRef || !constant || x.Method {
		return
	}
	c := &p.chains[idx]
	c.path = append(c.path, step)
	c.guarded = c.guarded || x.Optional
	ast.Patch(n, &ast.IdentifierNode{Value: refName(idx)})
}

func (p *refPatcher) visitBinary(n *ast.Node, x *ast.BinaryNode) {
	if x.Operator == "??" {
		if idx, ok := refOf(x.Left); ok {
			p.chains[idx].guarded = true
		}
		return
	}
	if x.Operator != "==" && x.Operator != "!=" {
		return
	}
	other, ok := emptyComparand(x)
	if !ok {
		return
	}
	var replacement ast.Node = &ast.CallNode{Callee: &ast.IdentifierNode{Value: isEmptyFunc}, Arguments: []ast.Node{other}}
	if x.Operator == "!=" {
		replacement = &ast.UnaryNode{Operator: "not", Node: replacement}
	}
	ast.Patch(n, replacement)
}

// emptyComparand returns the side of x compared against empty.
func emptyComparand(x *ast.BinaryNode) (ast.Node, bool) {
	if isEmptyIdent(x.Right) {
		return x.Left, true
	}
	if isEmptyIdent(x.Left) {
		return x.Right, true
	}
	return nil, false
}

func isEmptyIdent(n ast.Node) bool {
	id, ok := n.(*ast.IdentifierNode)
	return ok && id.Value == emptyIdent
}

func refName(i int) string { return refPrefix + strconv.Itoa(i) }

func refOf(n ast.Node) (int, bool) {
	id, ok := n.(*ast.IdentifierNode)
	if !ok || !strings.HasPrefix(id.Value, refPrefix) {
		return 0, false
	}
	i, err := strconv.Atoi(strings.TrimPrefix(id.Value, refPrefix))
	return i, err == nil
}

func constantStep(n ast.Node) (any, bool) {
	switch p := n.(type) {
	case *ast.StringNode:
		return p.Value, true
	case *ast.IntegerNode:
		return p.Value, true
	}
	return nil, false
}

// isEmpty is what "x == empty" means: null, "", or an empty list or map.
func isEmpty(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("empty comparison takes one value, got %d", len(args))
	}
	v := args[0]
	if v == nil {
		return true, nil
	}
	if s, ok := v.(string); ok {
		return s == "", nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0, nil
	}
	return false, nil
}
