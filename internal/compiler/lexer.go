package compiler

import (
	"fmt"
	"strings"
)

func lex(source string) ([]token, error) {
	var tokens []token
	line, column := 1, 1
	for index := 0; index < len(source); {
		current := source[index]
		if isSpace(current) {
			if current == '\n' {
				line, column = line+1, 1
			} else {
				column++
			}
			index++
			continue
		}
		if current == '-' && index+1 < len(source) && source[index+1] == '-' {
			index += 2
			column += 2
			for index < len(source) && source[index] != '\n' {
				index++
				column++
			}
			continue
		}
		if current == '(' && index+1 < len(source) && source[index+1] == '*' {
			startLine, startColumn := line, column
			index += 2
			column += 2
			depth := 1
			for index < len(source) && depth > 0 {
				if index+1 < len(source) && source[index] == '(' && source[index+1] == '*' {
					depth++
					index += 2
					column += 2
					continue
				}
				if index+1 < len(source) && source[index] == '*' && source[index+1] == ')' {
					depth--
					index += 2
					column += 2
					continue
				}
				if source[index] == '\n' {
					line, column = line+1, 1
				} else {
					column++
				}
				index++
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated block comment at line %d, column %d", startLine, startColumn)
			}
			continue
		}
		if isLower(current) {
			start := index
			startColumn := column
			for index < len(source) && isIdentifierPart(source[index]) {
				index++
				column++
			}
			tokens = append(tokens, token{kind: tokenIdentifier, text: source[start:index], line: line, column: startColumn})
			continue
		}
		if isDigit(current) {
			start := index
			startColumn := column
			for index < len(source) && isDigit(source[index]) {
				index++
				column++
			}
			tokens = append(tokens, token{kind: tokenNumber, text: source[start:index], line: line, column: startColumn})
			continue
		}
		startColumn := column
		if index+1 < len(source) {
			operator := source[index : index+2]
			switch operator {
			case "!!", "&&", "==", "!=", "<=", ">=":
				tokens = append(tokens, token{kind: tokenSymbol, text: operator, line: line, column: startColumn})
				index += 2
				column += 2
				continue
			}
		}
		if strings.ContainsRune("{}();=+-*/%<>", rune(current)) {
			tokens = append(tokens, token{kind: tokenSymbol, text: string(current), line: line, column: startColumn})
			index++
			column++
			continue
		}
		return nil, fmt.Errorf("unexpected character %q at line %d, column %d", current, line, column)
	}
	tokens = append(tokens, token{kind: tokenEOF, line: line, column: column})
	return tokens, nil
}
