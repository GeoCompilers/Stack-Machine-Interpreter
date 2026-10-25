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
	return p.list(false)
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

func (p *inputParser) list(array bool) ([]Value, error) {
	var result []Value
	if array && p.pos < len(p.text) && p.text[p.pos] == ']' {
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
		if array {
			if p.pos < len(p.text) && p.text[p.pos] == ']' {
				p.pos++
				return result, nil
			}
		} else if p.pos == len(p.text) {
			return result, nil
		}
		if p.pos == len(p.text) || p.text[p.pos] != ',' {
			return nil, p.fail("expected ','")
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
	case '[':
		p.pos++
		if err := p.space(); err != nil {
			return nil, err
		}
		elements, err := p.list(true)
		return &array{elements: elements}, err
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
	default:
		start := p.pos
		if p.text[p.pos] == '-' {
			p.pos++
		}
		digits := p.pos
		for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
			p.pos++
		}
		if digits == p.pos {
			return nil, p.fail("expected an integer, string or array")
		}
		n, err := strconv.ParseInt(p.text[start:p.pos], 10, 32)
		if err != nil {
			return nil, p.fail("integer outside int32 range")
		}
		return int32(n), nil
	}
}

func ReadInput(input io.Reader) ([]Value, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return parseInput(string(data))
}
