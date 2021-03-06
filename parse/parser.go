// Copyright 2016 Guilherme Nemeth <guilherme.nemeth@gmail.com>

package parse

import (
	"fmt"
	"github.com/glhrmfrts/yo/ast"
	"strconv"
)

type parser struct {
	tok            ast.Token
	literal        string
	ignoreNewlines bool
	tokenizer      tokenizer
}

type ParseError struct {
	Guilty  ast.Token
	Line    int
	File    string
	Message string
}

func (err *ParseError) Error() string {
	return fmt.Sprintf("%s:%d: %s", err.File, err.Line, err.Message)
}

//
// common productions
//

func parseNumber(typ ast.Token, str string) float64 {
	if typ == ast.TokenFloat {
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			panic(err)
		}
		return f
	} else {
		i, err := strconv.Atoi(str)
		if err != nil {
			panic(err)
		}
		return float64(i)
	}
}

func (p *parser) error(msg string) {
	t := p.tokenizer
	panic(&ParseError{Guilty: p.tok, Line: t.lineno, File: t.filename, Message: msg})
}

func (p *parser) errorExpected(expected string) {
	p.error(fmt.Sprintf("unexpected %s, expected %s", p.tok, expected))
}

func (p *parser) line() int {
	return p.tokenizer.lineno
}

func (p *parser) next() {
	p.tok, p.literal = p.tokenizer.nextToken()

	for p.ignoreNewlines && p.tok == ast.TokenNewline {
		p.tok, p.literal = p.tokenizer.nextToken()
	}
}

func (p *parser) accept(toktype ast.Token) bool {
	if p.tok == toktype {
		p.next()
		return true
	}
	return false
}

func (p *parser) makeId() *ast.Id {
	return &ast.Id{Value: p.literal}
}

func (p *parser) makeSelector(left ast.Node) *ast.Selector {
	return &ast.Selector{Left: left, Value: p.literal}
}

func (p *parser) idList() []*ast.Id {
	var list []*ast.Id

	for p.tok == ast.TokenId {
		list = append(list, &ast.Id{Value: p.literal})

		p.next()
		if !p.accept(ast.TokenComma) {
			break
		}
	}

	return list
}

// check if an expression list contains only identifiers
func (p *parser) checkIdList(list []ast.Node) bool {
	for _, node := range list {
		if _, isId := node.(*ast.Id); !isId {
			return false
		}
	}

	return true
}

// check if an expression can be at left side of an assignment
func (p *parser) checkLhs(node ast.Node) bool {
	switch node.(type) {
	case *ast.Id, *ast.Selector, *ast.Subscript:
		return true
	default:
		return false
	}
}

// same as above but with a list of expressions
func (p *parser) checkLhsList(list []ast.Node) bool {
	for _, node := range list {
		if !p.checkLhs(node) {
			return false
		}
	}

	return true
}

func (p *parser) exprList(inArray bool) []ast.Node {
	var list []ast.Node
	for {
		// trailing comma check
		if inArray && p.tok == ast.TokenRbrack {
			break
		}

		expr := p.expr()
		list = append(list, expr)
		if !p.accept(ast.TokenComma) {
			break
		}
	}

	return list
}

//
// grammar rules
//

func (p *parser) array() ast.Node {
	line := p.line()
	p.next() // '['

	if p.accept(ast.TokenRbrack) {
		// no elements
		return &ast.Array{}
	}

	list := p.exprList(true)
	if !p.accept(ast.TokenRbrack) {
		p.errorExpected("closing ']'")
	}

	return &ast.Array{Elements: list, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) objectFieldList() []*ast.ObjectField {
	var list []*ast.ObjectField
	for {
		// trailing comma check
		if p.tok == ast.TokenRbrace {
			break
		}

		var key string
		if p.tok == ast.TokenId || p.tok == ast.TokenString {
			key = p.literal
			p.next()
		} else {
			p.errorExpected("identifier or string")
		}

		line := p.line()
		if !p.accept(ast.TokenColon) {
			list = append(list, &ast.ObjectField{Key: key, NodeInfo: ast.NodeInfo{line}})
		} else {
			value := p.expr()
			list = append(list, &ast.ObjectField{Key: key, Value: value, NodeInfo: ast.NodeInfo{line}})
		}

		if !p.accept(ast.TokenComma) {
			break
		}
	}

	return list
}

func (p *parser) object() ast.Node {
	line := p.line()
	p.next() // '{'

	if p.accept(ast.TokenRbrace) {
		// no elements
		return &ast.Object{}
	}

	fields := p.objectFieldList()
	if !p.accept(ast.TokenRbrace) {
		p.errorExpected("closing '}'")
	}

	return &ast.Object{Fields: fields, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) functionArgs() []ast.Node {
	if !p.accept(ast.TokenLparen) {
		p.errorExpected("'('")
	}

	var list []ast.Node
	if p.accept(ast.TokenRparen) {
		// no arguments
		return list
	}

	var vararg, kwarg bool
	for p.tok == ast.TokenId {
		if vararg {
			p.error("argument after variadic argument")
		}

		var arg ast.Node
		line := p.line()
		id := p.makeId()
		p.next()

		// '='
		if p.accept(ast.TokenEq) {
			value := p.expr()
			arg = &ast.KwArg{Key: id.Value, Value: value, NodeInfo: ast.NodeInfo{line}}
			kwarg = true
		} else if p.accept(ast.TokenDotdotdot) {
			arg = &ast.VarArg{Arg: id, NodeInfo: ast.NodeInfo{line}}
			vararg = true
		} else {
			if vararg {
				p.error("positional argument after variadic argument")
			}
			if kwarg {
				p.error("positional argument after keyword argument")
			}
			arg = id
		}

		list = append(list, arg)
		if !p.accept(ast.TokenComma) {
			break
		}
	}

	if !p.accept(ast.TokenRparen) {
		p.errorExpected("closing ')'")
	}
	return list
}

func (p *parser) functionBody() ast.Node {
	line := p.line()
	if p.accept(ast.TokenTilde) {
		// '^' curried function
		args := p.functionArgs()
		body := p.functionBody()
		fn := &ast.Function{Args: args, Body: body, NodeInfo: ast.NodeInfo{line}}

		return &ast.Block{
			Nodes:    []ast.Node{&ast.ReturnStmt{Values: []ast.Node{fn}, NodeInfo: ast.NodeInfo{line}}},
			NodeInfo: ast.NodeInfo{line},
		}
	} else if p.accept(ast.TokenMinusgt) {
		// '->' short function
		list := p.exprList(false)

		return &ast.Block{
			Nodes:    []ast.Node{&ast.ReturnStmt{Values: list, NodeInfo: ast.NodeInfo{line}}},
			NodeInfo: ast.NodeInfo{line},
		}
	} else if p.tok == ast.TokenLbrace {
		// '{' regular function body
		return p.block()
	}

	p.errorExpected("'^', '=>' or '{'")
	return nil
}

func (p *parser) function() ast.Node {
	line := p.line()
	p.next() // 'func'

	var name ast.Node
	if p.tok != ast.TokenLparen {
		name = p.selectorOrSubscriptExpr(nil)
		if !p.checkLhs(name) {
			p.error("function name must be assignable")
		}
	}

	args := p.functionArgs()
	body := p.functionBody()
	return &ast.Function{Name: name, Args: args, Body: body, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) primaryExpr() ast.Node {
	line := p.line()
	// these first productions before the second 'switch'
	// handle the ending token themselves, so 'defer p.next()'
	// needs to be after them
	switch p.tok {
	case ast.TokenFunc:
		return p.function()
	case ast.TokenLbrack:
		return p.array()
	case ast.TokenLbrace:
		return p.object()
	case ast.TokenLparen:
		p.next()
		expr := p.expr()
		if !p.accept(ast.TokenRparen) {
			p.errorExpected("closing ')'")
		}

		return expr
	default:
		defer p.next()
		switch p.tok {
		case ast.TokenInt, ast.TokenFloat:
			return &ast.Number{Value: parseNumber(p.tok, p.literal), NodeInfo: ast.NodeInfo{line}}
		case ast.TokenId:
			return &ast.Id{Value: p.literal, NodeInfo: ast.NodeInfo{line}}
		case ast.TokenString:
			return &ast.String{Value: p.literal, NodeInfo: ast.NodeInfo{line}}
		case ast.TokenTrue, ast.TokenFalse:
			return &ast.Bool{Value: p.tok == ast.TokenTrue, NodeInfo: ast.NodeInfo{line}}
		case ast.TokenNil:
			return &ast.Nil{NodeInfo: ast.NodeInfo{line}}
		}
	}

	p.error(fmt.Sprintf("unexpected %s", p.tok))
	return nil
}

func (p *parser) selectorExpr(left ast.Node) ast.Node {
	if !(p.tok == ast.TokenId) {
		p.errorExpected("identifier")
	}

	defer p.next()
	return p.makeSelector(left)
}

func (p *parser) subscriptExpr(left ast.Node) ast.Node {
	line := p.line()
	expr := p.expr()
	sub := &ast.Subscript{Left: left, Right: expr}
	if p.accept(ast.TokenColon) {
		expr2 := p.expr()
		sub.Right = &ast.Slice{Start: expr, End: expr2, NodeInfo: ast.NodeInfo{line}}
	}

	if !p.accept(ast.TokenRbrack) {
		p.errorExpected("closing ']'")
	}

	return sub
}

func (p *parser) selectorOrSubscriptExpr(left ast.Node) ast.Node {
	if left == nil {
		left = p.primaryExpr()
	}

	for {
		if dot, lBrack := p.tok == ast.TokenDot, p.tok == ast.TokenLbrack; dot || lBrack {
			line := p.line()
			old := p.ignoreNewlines
			p.ignoreNewlines = false
			p.next()
			if p.tok == ast.TokenNewline || p.tok == ast.TokenEos {
				p.error("expression not terminated")
			}
			p.ignoreNewlines = old

			if dot {
				left = p.selectorExpr(left)
				left.(*ast.Selector).NodeInfo.Line = line
			} else {
				left = p.subscriptExpr(left)
				left.(*ast.Subscript).NodeInfo.Line = line
			}
		} else {
			break
		}
	}

	return left
}

func (p *parser) callArgs() []ast.Node {
	var list []ast.Node
	if p.tok == ast.TokenRparen {
		// no arguments
		return list
	}

	for {
		line := p.line()
		arg := p.expr()

		// '='
		if p.accept(ast.TokenEq) {
			value := p.expr()

			if id, isId := arg.(*ast.Id); isId {
				arg = &ast.KwArg{Key: id.Value, Value: value, NodeInfo: ast.NodeInfo{line}}
			} else {
				p.error("non-identifier in left side of keyword argument")
			}
		} else if p.accept(ast.TokenDotdotdot) {
			arg = &ast.VarArg{Arg: arg, NodeInfo: ast.NodeInfo{line}}
		}

		list = append(list, arg)
		if !p.accept(ast.TokenComma) {
			break
		}
	}

	return list
}

func (p *parser) callExpr() ast.Node {
	line := p.line()
	left := p.selectorOrSubscriptExpr(nil)

	var args []ast.Node
	for p.accept(ast.TokenLparen) {
		args = p.callArgs()
		if !p.accept(ast.TokenRparen) {
			p.errorExpected("closing ')'")
		}
		left = &ast.CallExpr{Left: left, Args: args, NodeInfo: ast.NodeInfo{line}}
	}

	return p.selectorOrSubscriptExpr(left)
}

func (p *parser) postfixExpr() ast.Node {
	line := p.line()
	left := p.callExpr()

	if ast.IsPostfixOp(p.tok) {
		op := p.tok
		p.next()
		return &ast.PostfixExpr{Op: op, Left: left, NodeInfo: ast.NodeInfo{line}}
	}

	return left
}

func (p *parser) unaryExpr() ast.Node {
	line := p.line()
	if ast.IsUnaryOp(p.tok) {
		op := p.tok
		p.next()

		var right ast.Node
		if op == ast.TokenNot {
			right = p.expr()
		} else {
			right = p.postfixExpr()
		}
		return &ast.UnaryExpr{Op: op, Right: right, NodeInfo: ast.NodeInfo{line}}
	}

	return p.postfixExpr()
}

// parse a binary expression using the legendary wikipedia's algorithm :)
func (p *parser) binaryExpr(left ast.Node, minPrecedence int) ast.Node {
	line := p.line()
	for ast.IsBinaryOp(p.tok) && ast.Precedence(p.tok) >= minPrecedence {
		op := p.tok
		opPrecedence := ast.Precedence(op)

		// consume operator
		old := p.ignoreNewlines
		p.ignoreNewlines = false
		p.next()
		if p.tok == ast.TokenNewline || p.tok == ast.TokenEos {
			p.error("expression not terminated")
		}
		p.ignoreNewlines = old

		right := p.unaryExpr()
		for (ast.IsBinaryOp(p.tok) && ast.Precedence(p.tok) > opPrecedence) ||
			(ast.RightAssociative(p.tok) && ast.Precedence(p.tok) >= opPrecedence) {
			right = p.binaryExpr(right, ast.Precedence(p.tok))
		}
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right, NodeInfo: ast.NodeInfo{line}}
	}

	return left
}

func (p *parser) ternaryExpr(left ast.Node) ast.Node {
	line := p.line()
	p.next() // '?'

	whenTrue := p.expr()
	if !p.accept(ast.TokenColon) {
		p.errorExpected("':'")
	}

	whenFalse := p.expr()
	return &ast.TernaryExpr{Cond: left, Then: whenTrue, Else: whenFalse, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) expr() ast.Node {
	left := p.binaryExpr(p.unaryExpr(), 0)

	// avoid unecessary calls to ternaryExpr
	if p.tok == ast.TokenQuestion {
		return p.ternaryExpr(left)
	}

	return left
}

func (p *parser) declaration() ast.Node {
	line := p.line()
	isConst := p.tok == ast.TokenConst
	p.next()

	left := p.idList()

	// '='
	if !p.accept(ast.TokenEq) {
		// a declaration without any values
		return &ast.Declaration{IsConst: isConst, Left: left, NodeInfo: ast.NodeInfo{line}}
	}

	right := p.exprList(false)
	return &ast.Declaration{IsConst: isConst, Left: left, Right: right, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) assignment(left []ast.Node) ast.Node {
	line := p.line()

	if left == nil {
		left = p.exprList(false)
	}

	if !ast.IsAssignOp(p.tok) {
		if len(left) > 1 {
			p.error("illegal expression")
		}
		return left[0]
	}

	// ':='
	if p.tok == ast.TokenColoneq {
		// a short variable declaration
		if isIdList := p.checkIdList(left); !isIdList {
			p.error("non-identifier at left side of ':='")
		}
	} else {
		// validate left side of assignment
		if isLhsList := p.checkLhsList(left); !isLhsList {
			p.error("non-assignable at left side of '='")
		}
	}

	op := p.tok
	p.next()

	right := p.exprList(false)
	return &ast.Assignment{Op: op, Left: left, Right: right, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) stmt() ast.Node {
	line := p.line()
	defer p.accept(ast.TokenSemicolon)
	switch tok := p.tok; tok {
	case ast.TokenConst, ast.TokenVar:
		return p.declaration()
	case ast.TokenBreak, ast.TokenContinue, ast.TokenFallthrough:
		p.next()
		return &ast.BranchStmt{Type: tok, NodeInfo: ast.NodeInfo{line}}
	case ast.TokenReturn:
		p.next()
		values := p.exprList(false)
		return &ast.ReturnStmt{Values: values, NodeInfo: ast.NodeInfo{line}}
	case ast.TokenPanic:
		p.next()
		err := p.expr()
		return &ast.PanicStmt{Err: err, NodeInfo: ast.NodeInfo{line}}
	case ast.TokenIf:
		return p.ifStmt()
	case ast.TokenFor:
		return p.forStmt()
	case ast.TokenTry:
		return p.tryRecoverStmt()
	default:
		return p.assignment(nil)
	}
}

func (p *parser) ifStmt() ast.Node {
	line := p.line()
	p.next() // 'if'

	var init *ast.Assignment
	var else_ ast.Node
	cond := p.assignment(nil)
	init, ok := cond.(*ast.Assignment)
	if ok {
		if !p.accept(ast.TokenSemicolon) {
			p.errorExpected("';'")
		}
		cond = p.expr()
	}

	body := p.block()
	if p.accept(ast.TokenElse) {
		if p.tok == ast.TokenLbrace {
			else_ = p.block()
		} else if p.tok == ast.TokenIf {
			else_ = p.ifStmt()
		} else {
			p.errorExpected("if or '{'")
		}
	}

	return &ast.IfStmt{Init: init, Cond: cond, Body: body, Else: else_, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) forIteratorStmt(ids []ast.Node) ast.Node {
	line := p.line()

	var key *ast.Id
	var value *ast.Id

	length := len(ids)
	if length > 2 {
		p.error("too many identifiers in for iterator statement")
	}

	ok := p.checkIdList(ids)
	if !ok {
		p.error("non-identifier at left-side of 'in' in for iterator statement")
	} else {
		key = ids[0].(*ast.Id)
		if length > 1 {
			value = ids[1].(*ast.Id)
		}
	}

	p.next() // 'in'
	coll := p.expr()

	var when ast.Node
	if p.accept(ast.TokenWhen) {
		when = p.expr()
	}

	body := p.block()
	return &ast.ForIteratorStmt{
		Key:        key,
		Value:      value,
		Collection: coll,
		When:       when,
		Body:       body,
		NodeInfo:   ast.NodeInfo{line},
	}
}

func (p *parser) forStmt() ast.Node {
	line := p.line()
	p.next() // 'for'

	var init *ast.Assignment
	var left []ast.Node
	var cond ast.Node
	var step ast.Node
	var ok bool
	if p.tok == ast.TokenLbrace {
		goto parseBody
	}

	left = p.exprList(false)
	if p.tok == ast.TokenIn {
		return p.forIteratorStmt(left)
	}

	cond = p.assignment(left)
	init, ok = cond.(*ast.Assignment)
	if ok {
		if !p.accept(ast.TokenSemicolon) {
			p.errorExpected("';'")
		}
		if p.tok == ast.TokenLbrace {
			cond = nil
			goto parseBody
		}
		cond = p.expr()
	}

	if p.accept(ast.TokenSemicolon) && p.tok != ast.TokenLbrace {
		step = p.assignment(nil)
	}

parseBody:
	body := p.block()
	return &ast.ForStmt{Init: init, Cond: cond, Step: step, Body: body, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) tryRecoverStmt() ast.Node {
	line := p.line()
	p.next() // 'try'

	tryBlock := p.block().(*ast.Block)

	var recoverBlock *ast.RecoverBlock
	if p.accept(ast.TokenRecover) {
		line := p.line()

		var id *ast.Id
		if p.tok == ast.TokenId {
			id = p.makeId()
			p.next()
		}

		block := p.block().(*ast.Block)
		recoverBlock = &ast.RecoverBlock{Id: id, Block: block, NodeInfo: ast.NodeInfo{line}}
	}

	var finallyBlock *ast.Block
	if p.accept(ast.TokenFinally) {
		finallyBlock = p.block().(*ast.Block)
	}

	return &ast.TryRecoverStmt{
		Try:      tryBlock,
		Recover:  recoverBlock,
		Finally:  finallyBlock,
		NodeInfo: ast.NodeInfo{line},
	}
}

func (p *parser) block() ast.Node {
	line := p.line()
	if !p.accept(ast.TokenLbrace) {
		p.errorExpected("'{'")
	}

	var nodes []ast.Node
	for !(p.tok == ast.TokenRbrace || p.tok == ast.TokenEos) {
		stmt := p.stmt()
		nodes = append(nodes, stmt)
	}

	if !p.accept(ast.TokenRbrace) {
		p.errorExpected("closing '}'")
	}
	return &ast.Block{Nodes: nodes, NodeInfo: ast.NodeInfo{line}}
}

func (p *parser) program() ast.Node {
	var nodes []ast.Node
	for !(p.tok == ast.TokenEos) {
		stmt := p.stmt()
		nodes = append(nodes, stmt)
	}

	return &ast.Block{Nodes: nodes}
}

// initialization of parser

func (p *parser) init(source []byte, filename string) {
	p.ignoreNewlines = true
	p.tokenizer.init(source, filename)

	// fetch the first token
	p.next()
}

func ParseExpr(source []byte) (expr ast.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			if perr, ok := r.(*ParseError); ok {
				err = perr
			} else {
				panic(r)
			}
		}
	}()

	var p parser
	p.init(source, "<expr>")
	expr = p.expr()
	return
}

func ParseFile(source []byte, filename string) (root ast.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			if perr, ok := r.(*ParseError); ok {
				err = perr
			} else {
				panic(r)
			}
		}
	}()

	var p parser
	p.init(source, filename)
	root = p.program()
	return
}
