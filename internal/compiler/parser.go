package compiler

import (
	"fmt"
	"strconv"
)

type parser struct {
	tokens []token
	index  int
}

func newParser(tokens []token) *parser {
	return &parser{tokens: tokens}
}

func (p *parser) current() token {
	return p.tokens[p.index]
}

func (p *parser) advance() token {
	current := p.current()
	if current.kind != tokenEOF {
		p.index++
	}
	return current
}

func (p *parser) match(text string) bool {
	if p.current().text != text {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(text string) (token, error) {
	current := p.current()
	if current.text != text {
		return token{}, fmt.Errorf("%s: expected %q, got %s", current.location(), text, describeToken(current))
	}
	p.advance()
	return current, nil
}

func (p *parser) expectIdentifier() (token, error) {
	current := p.current()
	if current.kind != tokenIdentifier || isKeyword(current.text) {
		return token{}, fmt.Errorf("%s: expected identifier, got %s", current.location(), describeToken(current))
	}
	p.advance()
	return current, nil
}

func (p *parser) parseProgram() (blockStatement, error) {
	if _, err := p.expect("{"); err != nil {
		return blockStatement{}, err
	}
	program, err := p.parseBlockContents()
	if err != nil {
		return blockStatement{}, err
	}
	if p.current().kind != tokenEOF {
		return blockStatement{}, fmt.Errorf("%s: expected end of source, got %s", p.current().location(), describeToken(p.current()))
	}
	return program, nil
}

func (p *parser) parseBlockContents() (blockStatement, error) {
	var statements []statement
	for p.current().text != "}" {
		if p.current().kind == tokenEOF {
			return blockStatement{}, fmt.Errorf("%s: expected %q before end of source", p.current().location(), "}")
		}
		statement, err := p.parseStatement()
		if err != nil {
			return blockStatement{}, err
		}
		statements = append(statements, statement)
	}
	if len(statements) == 0 {
		return blockStatement{}, fmt.Errorf("%s: block must contain at least one statement", p.current().location())
	}
	p.advance()
	return blockStatement{statements: statements}, nil
}

func (p *parser) parseStatement() (statement, error) {
	current := p.current()
	if current.text == "{" {
		p.advance()
		return p.parseBlockContents()
	}
	if current.kind != tokenIdentifier {
		return nil, fmt.Errorf("%s: expected statement, got %s", current.location(), describeToken(current))
	}
	var result statement
	var err error
	switch current.text {
	case "read":
		result, err = p.parseRead(true)
	case "write":
		result, err = p.parseWrite(true)
	case "while":
		result, err = p.parseWhile()
	case "do":
		result, err = p.parseDo()
	case "for":
		result, err = p.parseFor()
	case "if":
		result, err = p.parseIf()
	case "skip":
		p.advance()
		p.match(";")
		result = skipStatement{}
	case "else", "elif":
		return nil, fmt.Errorf("%s: %q has no matching if", current.location(), current.text)
	default:
		result, err = p.parseAssignment(true)
	}
	return result, err
}

func (p *parser) parseRead(semicolon bool) (statement, error) {
	p.advance()
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	name, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	if semicolon {
		p.match(";")
	}
	return readStatement{name: name.text}, nil
}

func (p *parser) parseWrite(semicolon bool) (statement, error) {
	p.advance()
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	value, err := p.parseExpression(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	if semicolon {
		p.match(";")
	}
	return writeStatement{value: value}, nil
}

func (p *parser) parseAssignment(semicolon bool) (statement, error) {
	name, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	operator := ""
	if !p.match("=") {
		current := p.current()
		if precedence(current.text) == 0 {
			return nil, fmt.Errorf("%s: expected assignment operator after %q", current.location(), name.text)
		}
		operator = current.text
		p.advance()
		if _, err := p.expect("="); err != nil {
			return nil, err
		}
	}
	value, err := p.parseExpression(1)
	if err != nil {
		return nil, err
	}
	if semicolon {
		p.match(";")
	}
	return assignmentStatement{name: name.text, operator: operator, value: value}, nil
}

func (p *parser) parseWhile() (statement, error) {
	p.advance()
	condition, err := p.parseParenthesizedExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return whileStatement{condition: condition, body: body}, nil
}

func (p *parser) parseDo() (statement, error) {
	p.advance()
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("while"); err != nil {
		return nil, err
	}
	condition, err := p.parseParenthesizedExpression()
	if err != nil {
		return nil, err
	}
	p.match(";")
	return doStatement{body: body, condition: condition}, nil
}

func (p *parser) parseFor() (statement, error) {
	p.advance()
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	initial, err := p.parseForClause()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(";"); err != nil {
		return nil, err
	}
	condition, err := p.parseExpression(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(";"); err != nil {
		return nil, err
	}
	post, err := p.parseForClause()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return forStatement{initial: initial, condition: condition, post: post, body: body}, nil
}

func (p *parser) parseForClause() (statement, error) {
	switch p.current().text {
	case "read":
		return p.parseRead(false)
	case "write":
		return p.parseWrite(false)
	case "skip":
		p.advance()
		return skipStatement{}, nil
	default:
		return p.parseAssignment(false)
	}
}

func (p *parser) parseIf() (statement, error) {
	p.advance()
	condition, err := p.parseParenthesizedExpression()
	if err != nil {
		return nil, err
	}
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	otherwise, err := p.parseElsePart()
	if err != nil {
		return nil, err
	}
	return ifStatement{condition: condition, then: then, otherwise: otherwise}, nil
}

func (p *parser) parseElsePart() (statement, error) {
	if p.match("else") {
		return p.parseStatement()
	}
	if !p.match("elif") {
		return nil, nil
	}
	condition, err := p.parseParenthesizedExpression()
	if err != nil {
		return nil, err
	}
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	otherwise, err := p.parseElsePart()
	if err != nil {
		return nil, err
	}
	return ifStatement{condition: condition, then: then, otherwise: otherwise}, nil
}

func (p *parser) parseParenthesizedExpression() (expression, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	value, err := p.parseExpression(1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *parser) parseExpression(minimumPrecedence int) (expression, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		operator := p.current().text
		level := precedence(operator)
		if level < minimumPrecedence {
			break
		}
		p.advance()
		right, err := p.parseExpression(level + 1)
		if err != nil {
			return nil, err
		}
		left = binaryExpression{operator: operator, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parsePrimary() (expression, error) {
	current := p.current()
	if current.kind == tokenIdentifier && !isKeyword(current.text) {
		p.advance()
		return variableExpression{name: current.text}, nil
	}
	if current.kind == tokenNumber {
		p.advance()
		return parseLiteral(current, false)
	}
	if p.match("-") {
		number := p.current()
		if number.kind != tokenNumber {
			return nil, fmt.Errorf("%s: expected a number after unary '-'", number.location())
		}
		p.advance()
		return parseLiteral(number, true)
	}
	if p.match("(") {
		value, err := p.parseExpression(1)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(")"); err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, fmt.Errorf("%s: expected expression, got %s", current.location(), describeToken(current))
}

func parseLiteral(token token, negative bool) (expression, error) {
	text := token.text
	if negative {
		text = "-" + text
	}
	value, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%s: constant %q is outside int32 range", token.location(), text)
	}
	return literalExpression{value: int32(value)}, nil
}

func precedence(operator string) int {
	switch operator {
	case "!!":
		return 1
	case "&&":
		return 2
	case "==", "!=":
		return 3
	case "<", "<=", ">", ">=":
		return 4
	case "+", "-":
		return 5
	case "*", "/", "%":
		return 6
	default:
		return 0
	}
}

func isKeyword(value string) bool {
	switch value {
	case "read", "write", "while", "do", "for", "if", "else", "elif", "skip":
		return true
	default:
		return false
	}
}
