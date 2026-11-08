package runtime

import (
	"fmt"
	"io"
	"strconv"
)

type inputParser struct {
	text string
	pos  int
}

func parseInput(text string) ([]Value, error) {
	p := inputParser{text: text}
	if err := p.space(); err != nil {
		return nil, err
	}
	if p.pos == len(text) {
		return nil, nil
	}
	return p.values(0, true)
}

func (p *inputParser) fail(message string) error {
	return fmt.Errorf("Input error: %s at byte %d", message, p.pos+1)
}

func (p *inputParser) space() error {
	for p.pos < len(p.text) {
		switch p.text[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
			continue
		}
		if p.starts("--") {
			for p.pos < len(p.text) && p.text[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		if p.starts("(*") {
			depth := 1
			p.pos += 2
			for depth > 0 {
				if p.pos == len(p.text) {
					return p.fail("unterminated comment")
				}
				if p.starts("(*") {
					depth++
					p.pos += 2
				} else if p.starts("*)") {
					depth--
					p.pos += 2
				} else {
					p.pos++
				}
			}
			continue
		}
		break
	}
	return nil
}
func (p *inputParser) starts(s string) bool {
	return len(p.text)-p.pos >= len(s) && p.text[p.pos:p.pos+len(s)] == s
}

func (p *inputParser) values(close byte, allowEmpty bool) ([]Value, error) {
	var result []Value
	if close != 0 && p.pos < len(p.text) && p.text[p.pos] == close {
		if !allowEmpty {
			return nil, p.fail("expected a value")
		}
		p.pos++
		return result, nil
	}
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		result = append(result, v)
		if err = p.space(); err != nil {
			return nil, err
		}
		if close != 0 && p.pos < len(p.text) && p.text[p.pos] == close {
			p.pos++
			return result, nil
		}
		if close == 0 && p.pos == len(p.text) {
			return result, nil
		}
		if p.pos == len(p.text) || p.text[p.pos] != ',' {
			return nil, p.fail("expected ',' or end of values")
		}
		p.pos++
		if err = p.space(); err != nil {
			return nil, err
		}
	}
}

func (p *inputParser) value() (Value, error) {
	if p.pos == len(p.text) {
		return nil, p.fail("expected a value")
	}
	switch p.text[p.pos] {
	case '[', '{':
		opener := p.text[p.pos]
		p.pos++
		if err := p.space(); err != nil {
			return nil, err
		}
		close := byte(']')
		if opener == '{' {
			close = '}'
		}
		elements, err := p.values(close, true)
		if err != nil {
			return nil, err
		}
		if opener == '[' {
			return NewArray(elements), nil
		}
		list := NewSExpression("nil", nil)
		for i := len(elements) - 1; i >= 0; i-- {
			list = NewSExpression("cons", []Value{elements[i], list})
		}
		return list, nil
	case '"':
		p.pos++
		var b []byte
		for p.pos < len(p.text) {
			current := p.text[p.pos]
			p.pos++
			if current == '"' {
				if p.pos < len(p.text) && p.text[p.pos] == '"' {
					p.pos++
					b = append(b, '"')
					continue
				}
				return b, nil
			}
			b = append(b, current)
		}
		return nil, p.fail("unterminated string")
	case '\'':
		return p.character()
	default:
		if p.text[p.pos] >= 'A' && p.text[p.pos] <= 'Z' {
			return p.constructor()
		}
		start := p.pos
		if p.text[p.pos] == '-' {
			p.pos++
		}
		digits := p.pos
		for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
			p.pos++
		}
		if digits == p.pos {
			return nil, p.fail("expected an integer, string, array, constructor, list or character")
		}
		n, err := strconv.ParseInt(p.text[start:p.pos], 10, 32)
		if err != nil {
			return nil, p.fail("integer outside int32 range")
		}
		return int32(n), nil
	}
}

func (p *inputParser) constructor() (Value, error) {
	start := p.pos
	p.pos++
	for p.pos < len(p.text) {
		c := p.text[p.pos]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			break
		}
		p.pos++
	}
	name := p.text[start:p.pos]
	if err := p.space(); err != nil {
		return nil, err
	}
	if p.pos == len(p.text) || p.text[p.pos] != '(' {
		return NewSExpression(name, nil), nil
	}
	p.pos++
	if err := p.space(); err != nil {
		return nil, err
	}
	args, err := p.values(')', false)
	if err != nil {
		return nil, err
	}
	return NewSExpression(name, args), nil
}

func (p *inputParser) character() (Value, error) {
	p.pos++
	if p.pos == len(p.text) {
		return nil, p.fail("unterminated character")
	}
	var ch byte
	switch {
	case p.starts("''"):
		ch = '\''
		p.pos += 2
	case p.starts(`\n`):
		ch = '\n'
		p.pos += 2
	case p.starts(`\t`):
		ch = '\t'
		p.pos += 2
	default:
		ch = p.text[p.pos]
		if ch == '\'' {
			return nil, p.fail("expected one character")
		}
		p.pos++
	}
	if p.pos == len(p.text) || p.text[p.pos] != '\'' {
		return nil, p.fail("expected closing character quote")
	}
	p.pos++
	return int32(ch), nil
}

func ReadInput(input io.Reader) ([]Value, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return parseInput(string(data))
}
