package gen

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Node is the interface that all AST nodes implement.
type Node interface {
	Emit(indent, target string) string
}

// TextNode holds plain text.
type TextNode struct {
	Text string
}

var rePlaceholder = regexp.MustCompile(`@@table|@@[A-Za-z0-9_.]+|@[A-Za-z0-9_.]+`)

func (t *TextNode) Emit(indent, target string) string {
	str := strings.TrimSpace(t.Text)
	if str == "" {
		return ""
	}

	escapedToken := "___ESCAPED_AT___"
	str = strings.ReplaceAll(str, "\\@", escapedToken)

	var params []string
	replaced := rePlaceholder.ReplaceAllStringFunc(str, func(ph string) string {
		switch {
		case ph == "@@table":
			params = append(params, "clause.CurrentTable")
			return "?"
		case strings.HasPrefix(ph, "@@"):
			params = append(params, fmt.Sprintf("gorm.Expr(\"?\", %s)", ph[2:]))
			return "?"
		case strings.HasPrefix(ph, "@"):
			params = append(params, ph[1:])
			return "?"
		}
		return ph
	})

	replaced = strings.ReplaceAll(replaced, escapedToken, "@")
	replaced = strings.ReplaceAll(replaced, "\"", "\\\"")

	var out strings.Builder
	out.WriteString(fmt.Sprintf("%sfmt.Fprintln(&%s, %q)\n", indent, target, replaced))
	if len(params) > 0 {
		out.WriteString(fmt.Sprintf("%sparams = append(params, %s)\n", indent, strings.Join(params, ", ")))
	}
	return out.String()
}

// FuncNode for {{where}} / {{set}} blocks.
type FuncNode struct {
	Name string
	Body []Node
}

func (f *FuncNode) Emit(indent, target string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", indent))
	b.WriteString(fmt.Sprintf("%s\tvar tmp strings.Builder\n", indent))
	for _, c := range f.Body {
		b.WriteString(c.Emit(indent+"\t", "tmp"))
	}
	b.WriteString(fmt.Sprintf("%s\tc := strings.TrimSpace(tmp.String())\n", indent))
	b.WriteString(fmt.Sprintf("%s\tif c != \"\" {\n", indent))
	switch f.Name {
	case "where":
		b.WriteString(fmt.Sprintf("%s\t\tfmt.Fprintln(&%s, \"WHERE \")\n", indent, target))
		b.WriteString(fmt.Sprintf("%s\t\tif len(c) >= 3 && strings.EqualFold(c[len(c)-3:], \"AND\") {\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t\tc = strings.TrimSpace(c[:len(c)-3])\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t} else if len(c) >= 2 && strings.EqualFold(c[len(c)-2:], \"OR\") {\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t\tc = strings.TrimSpace(c[:len(c)-2])\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\tfmt.Fprintln(&%s, \"WHERE \")\n", indent, target))
		b.WriteString(fmt.Sprintf("%s\t\tfmt.Fprintln(&%s, c)\n", indent, target))
	case "set":
		b.WriteString(fmt.Sprintf("%s\t\tif strings.HasSuffix(c, \",\") {\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t\tc = strings.TrimSpace(strings.TrimRight(c, \",\"))\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\t}\n", indent))
		b.WriteString(fmt.Sprintf("%s\t\tfmt.Fprintln(&%s, \"SET \")\n", indent, target))
		b.WriteString(fmt.Sprintf("%s\t\tfmt.Fprintln(&%s, c)\n", indent, target))
	default:
		panic(fmt.Sprintf("unsupported func %q in sql tempalte\n", f.Name))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", indent))
	b.WriteString(fmt.Sprintf("%s}\n", indent))
	return b.String()
}

// ForNode for {{for expr}}.
type ForNode struct {
	Expr string
	Body []Node
}

func (fn *ForNode) Emit(indent, target string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sfor %s {\n", indent, fn.Expr))
	for _, c := range fn.Body {
		b.WriteString(c.Emit(indent+"\t", target))
	}
	b.WriteString(fmt.Sprintf("%s}\n", indent))
	return b.String()
}

// IfBranch holds one condition + body.
type IfBranch struct {
	Cond string
	Body []Node
}

// IfNode can have multiple branches (if, else if, else if, ...), plus an optional else.
type IfNode struct {
	Branches []IfBranch
	ElseBody []Node
}

func (in *IfNode) Emit(indent, target string) string {
	var b strings.Builder
	// if branches[0].Cond { ... } else if branches[1].Cond { ... } else ...
	for i, br := range in.Branches {
		if i == 0 {
			b.WriteString(fmt.Sprintf("%sif %s {\n", indent, br.Cond))
		} else {
			b.WriteString(fmt.Sprintf("%s} else if %s {\n", indent, br.Cond))
		}
		for _, c := range br.Body {
			b.WriteString(c.Emit(indent+"\t", target))
		}
	}
	if len(in.ElseBody) > 0 {
		b.WriteString(fmt.Sprintf("%s} else {\n", indent))
		for _, c := range in.ElseBody {
			b.WriteString(c.Emit(indent+"\t", target))
		}
	}
	b.WriteString(fmt.Sprintf("%s}\n", indent))
	return b.String()
}

// stackItem holds a node or ifNode under construction.
type stackItem struct {
	node      Node
	ifNode    *IfNode // non-nil if it's an if
	branchIdx int     // which branch index are we currently filling?
	elsePart  bool
}

// RenderSQLTemplate parses the template string and returns Go code or an error.
func RenderSQLTemplate(tmpl string) (string, error) {
	var root []Node
	var stack []stackItem

	// getBody returns the Node slice we should append text/child-block to,
	// depending on if we're in an if branch or else part, or a for/func block
	getBody := func(si *stackItem) *[]Node {
		if si.ifNode == nil {
			// for, funcNode
			switch x := si.node.(type) {
			case *FuncNode:
				return &x.Body
			case *ForNode:
				return &x.Body
			}
			return nil
		}
		// ifNode
		if si.elsePart {
			return &si.ifNode.ElseBody
		}
		// normal branch
		return &si.ifNode.Branches[si.branchIdx].Body
	}

	appendText := func(txt string) {
		str := strings.TrimSpace(txt)
		if str == "" {
			return
		}
		t := &TextNode{Text: txt}
		if len(stack) == 0 {
			root = append(root, t)
			return
		}
		top := &stack[len(stack)-1]
		b := getBody(top)
		*b = append(*b, t)
	}

	pushBlock := func(n Node) {
		// push a non-if block (for, func)
		if len(stack) == 0 {
			stack = append(stack, stackItem{node: n})
		} else {
			top := &stack[len(stack)-1]
			b := getBody(top)
			*b = append(*b, n)
			stack = append(stack, stackItem{node: n})
		}
	}

	handleIfStart := func(cond string) {
		in := &IfNode{
			Branches: []IfBranch{
				{Cond: cond},
			},
		}
		if len(stack) == 0 {
			stack = append(stack, stackItem{node: in, ifNode: in, branchIdx: 0})
		} else {
			top := &stack[len(stack)-1]
			b := getBody(top)
			*b = append(*b, in)
			stack = append(stack, stackItem{node: in, ifNode: in, branchIdx: 0})
		}
	}

	handleElseIf := func(cond string) error {
		if len(stack) == 0 {
			return errors.New("else if without an open if block")
		}
		top := &stack[len(stack)-1]
		if top.ifNode == nil {
			return errors.New("else if outside if block")
		}
		if top.elsePart {
			return errors.New("else if after else")
		}
		// add a new branch
		in := top.ifNode
		in.Branches = append(in.Branches, IfBranch{Cond: cond})
		top.branchIdx = len(in.Branches) - 1
		return nil
	}

	handleElse := func() error {
		if len(stack) == 0 {
			return errors.New("else without if")
		}
		top := &stack[len(stack)-1]
		if top.ifNode == nil {
			return errors.New("else outside if block")
		}
		if top.elsePart {
			return errors.New("multiple else in same if block")
		}
		top.elsePart = true
		return nil
	}

	handleEnd := func() error {
		if len(stack) == 0 {
			return errors.New("unmatched end")
		}
		finished := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			root = append(root, finished.node)
		}
		return nil
	}

	handleDirective := func(dir string, lineNo int) error {
		switch {
		case dir == "where" || dir == "set":
			fn := &FuncNode{Name: dir}
			pushBlock(fn)
		case strings.HasPrefix(dir, "for "):
			ex := strings.TrimSpace(dir[3:])
			f := &ForNode{Expr: ex}
			pushBlock(f)
		case strings.HasPrefix(dir, "if "):
			c := strings.TrimSpace(dir[2:])
			handleIfStart(c)
		case strings.HasPrefix(dir, "else if "):
			c := strings.TrimSpace(dir[len("else if "):])
			return handleElseIf(c)
		case dir == "else":
			return handleElse()
		case dir == "end":
			return handleEnd()
		default:
			return fmt.Errorf("unknown directive: %q (line %d)", dir, lineNo)
		}
		return nil
	}

	lines := strings.Split(tmpl, "\n")
	for i, line := range lines {
		rest := line
		for {
			start := strings.Index(rest, "{{")
			if start == -1 {
				appendText(rest)
				break
			}
			if start > 0 {
				appendText(rest[:start])
			}
			rest = rest[start+2:]
			end := strings.Index(rest, "}}")
			if end == -1 {
				return "", fmt.Errorf("line %d: missing }}", i+1)
			}
			dir := strings.TrimSpace(rest[:end])
			rest = rest[end+2:]
			if err := handleDirective(dir, i+1); err != nil {
				return "", fmt.Errorf("line %d: %w", i+1, err)
			}
		}
	}
	if len(stack) > 0 {
		return "", errors.New("unclosed block(s) at EOF")
	}

	var (
		sb          strings.Builder
		codes       []string
		paramsCount int
	)

	for _, n := range root {
		code := n.Emit("", "sb")
		count, baseCount := 0, 1

		for _, line := range strings.Split(code, "\n") {
			if strings.Index(code, "\tfor ") > 0 {
				baseCount = 4
			}
			if strings.Contains(line, "params = append(params") {
				count += strings.Count(line, ",") * baseCount
			}
		}

		paramsCount += count
		codes = append(codes, n.Emit("", "sb"))
	}

	sb.WriteString("var sb strings.Builder\n")
	sb.WriteString(fmt.Sprintf("params := make([]any, 0, %d)\n\n", paramsCount))

	for _, code := range codes {
		sb.WriteString(code)
	}
	return sb.String(), nil
}
