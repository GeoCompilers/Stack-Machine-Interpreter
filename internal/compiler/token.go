package compiler

import (
	"fmt"
	"strconv"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenNumber
	tokenSymbol
)

type token struct {
	kind   tokenKind
	text   string
	line   int
	column int
}

func (t token) location() string {
	return fmt.Sprintf("line %d, column %d", t.line, t.column)
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isLower(value byte) bool {
	return value >= 'a' && value <= 'z'
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func isIdentifierPart(value byte) bool {
	return isLower(value) || (value >= 'A' && value <= 'Z') || isDigit(value) || value == '_' || value == '\''
}

func describeToken(token token) string {
	if token.kind == tokenEOF {
		return "end of source"
	}
	return strconv.Quote(token.text)
}
