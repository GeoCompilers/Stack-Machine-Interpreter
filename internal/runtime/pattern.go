package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Pattern struct {
	kind     string
	text     string
	number   int32
	inner    *Pattern
	elements []*Pattern
}

func patternObject(data json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil || len(fields) != len(keys) {
		return nil, fmt.Errorf("AST: pattern expected fields %v", keys)
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return nil, fmt.Errorf("AST: pattern missing field %q", key)
		}
	}
	return fields, nil
}

func patternName(data json.RawMessage) (string, error) {
	var name string
	if err := json.Unmarshal(data, &name); err != nil || name == "" {
		return "", fmt.Errorf("AST: pattern requires a nonempty name")
	}
	return name, nil
}

func patternElements(data json.RawMessage) ([]*Pattern, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return nil, fmt.Errorf("AST: pattern requires an array")
	}
	elements := make([]*Pattern, len(raw))
	for i, item := range raw {
		var err error
		elements[i], err = DecodePattern(item)
		if err != nil {
			return nil, err
		}
	}
	return elements, nil
}

func DecodePattern(data json.RawMessage) (*Pattern, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("AST: expected a pattern")
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return nil, fmt.Errorf("AST: invalid string pattern: %w", err)
		}
		kind := "string"
		switch text {
		case "#int", "#string", "#array", "#sexp":
			kind = text
		}
		return &Pattern{kind: kind, text: text}, nil
	}
	if data[0] == '-' || data[0] >= '0' && data[0] <= '9' {
		var number int32
		if err := json.Unmarshal(data, &number); err != nil {
			return nil, fmt.Errorf("AST: invalid integer pattern: %w", err)
		}
		return &Pattern{kind: "const", number: number}, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("AST: expected a pattern object, string or int32")
	}
	if fields["sexp"] != nil {
		fields, err := patternObject(data, "sexp", "args")
		if err != nil {
			return nil, err
		}
		name, err := patternName(fields["sexp"])
		if err != nil {
			return nil, err
		}
		elements, err := patternElements(fields["args"])
		if err != nil {
			return nil, err
		}
		return &Pattern{kind: "sexp", text: name, elements: elements}, nil
	}
	if len(fields) != 1 {
		return nil, fmt.Errorf("AST: expected one pattern kind")
	}
	for kind, payload := range fields {
		switch kind {
		case "wildcard":
			var empty []json.RawMessage
			if err := json.Unmarshal(payload, &empty); err != nil || empty == nil || len(empty) != 0 {
				return nil, fmt.Errorf("AST: wildcard requires []")
			}
			return &Pattern{kind: kind}, nil
		case "array":
			elements, err := patternElements(payload)
			if err != nil {
				return nil, err
			}
			return &Pattern{kind: kind, elements: elements}, nil
		case "bind":
			binding, err := patternObject(payload, "pattern", "name")
			if err != nil {
				return nil, err
			}
			name, err := patternName(binding["name"])
			if err != nil {
				return nil, err
			}
			inner, err := DecodePattern(binding["pattern"])
			if err != nil {
				return nil, err
			}
			return &Pattern{kind: kind, text: name, inner: inner}, nil
		default:
			return nil, fmt.Errorf("AST: unknown pattern %q", kind)
		}
	}
	return nil, fmt.Errorf("AST: expected a pattern")
}

func Match(pattern *Pattern, value Value) (map[string]Value, bool) {
	bindings := make(map[string]Value)
	if !match(pattern, value, bindings) {
		return nil, false
	}
	return bindings, true
}

func match(pattern *Pattern, value Value, bindings map[string]Value) bool {
	if pattern == nil {
		return false
	}
	var elements []Value
	switch pattern.kind {
	case "wildcard":
		return true
	case "bind":
		if !match(pattern.inner, value, bindings) {
			return false
		}
		bindings[pattern.text] = value
		return true
	case "const":
		number, ok := value.(int32)
		return ok && number == pattern.number
	case "string":
		text, ok := value.([]byte)
		return ok && bytes.Equal(text, []byte(pattern.text))
	case "#int":
		_, ok := value.(int32)
		return ok
	case "#string":
		_, ok := value.([]byte)
		return ok
	case "#array":
		_, ok := value.(*array)
		return ok
	case "#sexp":
		_, ok := value.(*sexpression)
		return ok
	case "array":
		object, ok := value.(*array)
		if !ok {
			return false
		}
		elements = object.elements
	case "sexp":
		object, ok := value.(*sexpression)
		if !ok || object.name != pattern.text {
			return false
		}
		elements = object.args
	default:
		return false
	}
	if len(elements) != len(pattern.elements) {
		return false
	}
	for i, child := range pattern.elements {
		if !match(child, elements[i], bindings) {
			return false
		}
	}
	return true
}

func MatchFailure(value Value) error {
	text, err := formatted(value)
	if err != nil {
		return err
	}
	return fmt.Errorf("ERROR: match failure on %s", text)
}
