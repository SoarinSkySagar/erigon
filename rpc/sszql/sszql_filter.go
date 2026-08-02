package sszql

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func compareRaw(a, b Raw) int {
	if len(a) < len(b) {
		a = append(make(Raw, len(b)-len(a)), a...)
	} else if len(b) < len(a) {
		b = append(make(Raw, len(a)-len(b)), b...)
	}
	return bytes.Compare(a, b)
}

func rawEqual(a, b Raw) bool { return compareRaw(a, b) == 0 }

var trueRaw = Raw{0x01}
var falseRaw = Raw{0x00}

func boolToRaw(b bool) Raw {
	if b {
		return trueRaw
	}
	return falseRaw
}

func rawToBool(r Raw) bool {
	for _, b := range r {
		if b != 0 {
			return true
		}
	}
	return false
}

func applyComparison(left []Raw, op string, right []Raw) ([]Raw, error) {
	if len(left) == 0 || len(right) == 0 {
		return nil, fmt.Errorf("applyComparison: empty operand for op %q", op)
	}

	if op == "in" {
		results := make([]Raw, len(left))
		for i, l := range left {
			found := false
			for _, r := range right {
				if rawEqual(l, r) {
					found = true
					break
				}
			}
			results[i] = boolToRaw(found)
		}
		return results, nil
	}

	ordering := op == ">" || op == "<" || op == ">=" || op == "<="
	leftIsArray := len(left) > 1
	rightIsArray := len(right) > 1

	if ordering && leftIsArray && rightIsArray {
		return nil, fmt.Errorf(
			"applyComparison: %q not supported between two arrays (left len=%d, right len=%d)",
			op, len(left), len(right),
		)
	}

	switch {
	case leftIsArray && rightIsArray:
		if len(left) != len(right) {
			return nil, fmt.Errorf(
				"applyComparison: array length mismatch for %q (left=%d, right=%d)",
				op, len(left), len(right),
			)
		}
		results := make([]Raw, len(left))
		for i := range left {
			res, err := compareScalar(left[i], op, right[i])
			if err != nil {
				return nil, err
			}
			results[i] = res
		}
		return results, nil

	case leftIsArray && !rightIsArray:
		results := make([]Raw, len(left))
		for i := range left {
			res, err := compareScalar(left[i], op, right[0])
			if err != nil {
				return nil, err
			}
			results[i] = res
		}
		return results, nil

	case !leftIsArray && rightIsArray:
		results := make([]Raw, len(right))
		for i := range right {
			res, err := compareScalar(left[0], op, right[i])
			if err != nil {
				return nil, err
			}
			results[i] = res
		}
		return results, nil

	default:
		res, err := compareScalar(left[0], op, right[0])
		if err != nil {
			return nil, err
		}
		return []Raw{res}, nil
	}
}

func compareScalar(l Raw, op string, r Raw) (Raw, error) {
	c := compareRaw(l, r)
	switch op {
	case "==":
		return boolToRaw(c == 0), nil
	case "!=":
		return boolToRaw(c != 0), nil
	case ">":
		return boolToRaw(c > 0), nil
	case "<":
		return boolToRaw(c < 0), nil
	case ">=":
		return boolToRaw(c >= 0), nil
	case "<=":
		return boolToRaw(c <= 0), nil
	default:
		return nil, fmt.Errorf("compareScalar: unsupported operator %q", op)
	}
}

func applyLogical(left []Raw, op string, right []Raw) ([]Raw, error) {
	if op != "&&" && op != "||" {
		return nil, fmt.Errorf("applyLogical: unsupported operator %q", op)
	}
	if len(left) == 0 || len(right) == 0 {
		return nil, fmt.Errorf("applyLogical: empty operand for op %q", op)
	}

	logic := func(a, b bool) bool {
		if op == "&&" {
			return a && b
		}
		return a || b
	}

	leftIsArray := len(left) > 1
	rightIsArray := len(right) > 1

	switch {
	case leftIsArray && rightIsArray:
		if len(left) != len(right) {
			return nil, fmt.Errorf(
				"applyLogical: array length mismatch for %q (left=%d, right=%d)",
				op, len(left), len(right),
			)
		}
		results := make([]Raw, len(left))
		for i := range left {
			results[i] = boolToRaw(logic(rawToBool(left[i]), rawToBool(right[i])))
		}
		return results, nil

	case leftIsArray && !rightIsArray:
		rb := rawToBool(right[0])
		results := make([]Raw, len(left))
		for i := range left {
			results[i] = boolToRaw(logic(rawToBool(left[i]), rb))
		}
		return results, nil

	case !leftIsArray && rightIsArray:
		lb := rawToBool(left[0])
		results := make([]Raw, len(right))
		for i := range right {
			results[i] = boolToRaw(logic(lb, rawToBool(right[i])))
		}
		return results, nil

	default:
		return []Raw{boolToRaw(logic(rawToBool(left[0]), rawToBool(right[0])))}, nil
	}
}

func (n *BinaryNode) Eval(ctx *EvalContext) ([]Raw, error) {
	left, err := n.Left.Eval(ctx)
	if err != nil {
		return nil, err
	}
	right, err := n.Right.Eval(ctx)
	if err != nil {
		return nil, err
	}
	if n.Op == "&&" || n.Op == "||" {
		return applyLogical(left, n.Op, right)
	}
	return applyComparison(left, n.Op, right)
}

func (n *NotNode) Eval(ctx *EvalContext) ([]Raw, error) {
	vals, err := n.Operand.Eval(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Raw, len(vals))
	for i, v := range vals {
		out[i] = boolToRaw(!rawToBool(v))
	}
	return out, nil
}

func (n *ListNode) Eval(ctx *EvalContext) ([]Raw, error) {
	var out []Raw
	for _, e := range n.Elems {
		vals, err := e.Eval(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, vals...)
	}
	return out, nil
}

func (n *LiteralNode) Eval(_ *EvalContext) ([]Raw, error) {
	return []Raw{n.Value}, nil
}

func (n *PathNode) Eval(ctx *EvalContext) ([]Raw, error) {
	return getValueFromPath(n.Path), nil
}

func (n *AliasNode) Eval(ctx *EvalContext) ([]Raw, error) {
	expansion, ok := ctx.Aliases[n.Name]
	if !ok {
		return nil, fmt.Errorf("eval: unknown alias $%s", n.Name)
	}
	return convertToRaw(expansion), nil
}

func precedenceOf(tok string) int {
	switch tok {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=", ">", "<", ">=", "<=", "in":
		return 3
	}
	return 0
}

func isComparison(tok string) bool { return precedenceOf(tok) == 3 }

func (p *Parser) peek() (string, bool) {
	if p.pos >= len(p.tokens) {
		return "", false
	}
	return p.tokens[p.pos], true
}

func (p *Parser) next() (string, bool) {
	tok, ok := p.peek()
	if ok {
		p.pos++
	}
	return tok, ok
}

func Parse(filter Filter) (Node, error) {

	var tokenPattern = regexp.MustCompile(
		`\(|\)|\[|\]|,` +
			`|==|!=|>=|<=|&&|\|\|` +
			`|!|>|<` +
			`|\bin\b` +
			`|\$[a-zA-Z_][a-zA-Z0-9_]*` +
			`|(?:\.[a-zA-Z_][a-zA-Z0-9_]*(?:\[[0-9]+\])?)+` +
			`|0x[a-fA-F0-9]+` +
			`|[0-9]+`,
	)

	tokens := tokenPattern.FindAllString(string(filter), -1)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("parse: empty or unrecognizable filter %q", filter)
	}
	p := &Parser{tokens: tokens}
	node, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, fmt.Errorf("parse: unexpected token %q at index %d", p.tokens[p.pos], p.pos)
	}
	return node, nil
}

func (p *Parser) parseExpr(minPrec int) (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		op, ok := p.peek()
		if !ok {
			break
		}
		prec := precedenceOf(op)
		if prec == 0 || prec < minPrec {
			break
		}
		p.next()

		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}

		if isComparison(op) {
			if nextTok, ok := p.peek(); ok && isComparison(nextTok) {
				return nil, fmt.Errorf(
					"parse: chained comparison %q %q — wrap in parentheses if intentional",
					op, nextTok,
				)
			}
		}

		left = &BinaryNode{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Node, error) {
	if tok, ok := p.peek(); ok && tok == "!" {
		p.next()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &NotNode{Operand: operand}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Node, error) {
	tok, ok := p.next()
	if !ok {
		return nil, fmt.Errorf("parse: unexpected end of filter")
	}

	switch tok {
	case "(":
		node, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		closing, ok := p.next()
		if !ok || closing != ")" {
			return nil, fmt.Errorf("parse: missing closing ')'")
		}
		return node, nil

	case "[":
		return p.parseList()

	case ")", "]", ",":
		return nil, fmt.Errorf("parse: unexpected %q at index %d", tok, p.pos-1)
	}

	return p.parseOperand(tok)
}

func (p *Parser) parseList() (Node, error) {
	list := &ListNode{}

	if tok, ok := p.peek(); ok && tok == "]" {
		p.next()
		return nil, fmt.Errorf("parse: empty list literal")
	}

	for {
		tok, ok := p.next()
		if !ok {
			return nil, fmt.Errorf("parse: unterminated list literal")
		}
		elem, err := p.parseOperand(tok)
		if err != nil {
			return nil, err
		}
		list.Elems = append(list.Elems, elem)

		sep, ok := p.next()
		if !ok {
			return nil, fmt.Errorf("parse: unterminated list literal")
		}
		if sep == "]" {
			return list, nil
		}
		if sep != "," {
			return nil, fmt.Errorf("parse: expected ',' or ']' in list, got %q", sep)
		}
	}
}

func (p *Parser) parseOperand(tok string) (Node, error) {
	runes := []rune(tok)
	switch {
	case runes[0] == '$':
		return &AliasNode{Name: string(runes[1:])}, nil

	case runes[0] == '.':
		return &PathNode{Path: Path(tok)}, nil

	case strings.HasPrefix(tok, "0x"):
		body := tok[2:]
		if len(body)%2 == 1 {
			body = "0" + body
		}
		b, err := hex.DecodeString(body)
		if err != nil {
			return nil, fmt.Errorf("parse: bad hex literal %q: %w", tok, err)
		}
		return &LiteralNode{Value: Raw(b)}, nil

	default:
		num, err := strconv.ParseUint(tok, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse: bad numeric literal %q: %w", tok, err)
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, num)
		return &LiteralNode{Value: Raw(buf)}, nil
	}
}
