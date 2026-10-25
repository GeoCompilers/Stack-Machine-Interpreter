package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"stack-machine-interpreter/internal/runtime"
)

type fields map[string]json.RawMessage

func object(data json.RawMessage) (fields, error) {
	var f fields
	if err := json.Unmarshal(data, &f); err != nil || f == nil {
		return nil, fmt.Errorf("AST: expected an object")
	}
	return f, nil
}

func (f fields) require(keys ...string) error {
	if len(f) != len(keys) {
		return fmt.Errorf("AST: expected fields %v", keys)
	}
	for _, key := range keys {
		if _, ok := f[key]; !ok {
			return fmt.Errorf("AST: missing field %q", key)
		}
	}
	return nil
}

func scalar[T any](data json.RawMessage) (T, error) {
	var v T
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return v, fmt.Errorf("AST: null is not a value")
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("AST: invalid scalar: %w", err)
	}
	return v, nil
}

func variableName(data json.RawMessage) (string, error) {
	name, err := scalar[string](data)
	if err == nil && name == "" {
		err = fmt.Errorf("AST: variable name must not be empty")
	}
	return name, err
}

func decodeExpression(data json.RawMessage) (expression, error) {
	f, err := object(data)
	if err != nil {
		return nil, err
	}
	if f["binop"] != nil {
		if err := f.require("binop", "left", "right"); err != nil {
			return nil, err
		}
		op, err := scalar[string](f["binop"])
		if err != nil {
			return nil, err
		}
		if !runtime.ValidOperator(op) {
			return nil, fmt.Errorf("AST: unknown operator %q", op)
		}
		left, err := decodeExpression(f["left"])
		if err != nil {
			return nil, err
		}
		right, err := decodeExpression(f["right"])
		return binaryExpression{operator: op, left: left, right: right}, err
	}
	if f["elem"] != nil {
		if err := f.require("elem", "index"); err != nil {
			return nil, err
		}
		object, err := decodeExpression(f["elem"])
		if err != nil {
			return nil, err
		}
		index, err := decodeExpression(f["index"])
		return elementExpression{object: object, index: index}, err
	}
	if len(f) != 1 {
		return nil, fmt.Errorf("AST: expected one expression kind")
	}
	for kind, payload := range f {
		switch kind {
		case "const":
			number, err := scalar[int32](payload)
			return literalExpression{value: number}, err
		case "var":
			name, err := variableName(payload)
			return variableExpression{name: name}, err
		case "string":
			text, err := scalar[string](payload)
			return stringExpression{text: text}, err
		case "length":
			object, err := decodeExpression(payload)
			return lengthExpression{object: object}, err
		case "array":
			raw, err := scalar[[]json.RawMessage](payload)
			if err != nil {
				return nil, err
			}
			elements := make([]expression, len(raw))
			for i, item := range raw {
				elements[i], err = decodeExpression(item)
				if err != nil {
					return nil, err
				}
			}
			return arrayExpression{elements: elements}, nil
		default:
			return nil, fmt.Errorf("AST: unknown expression %q", kind)
		}
	}
	panic("unreachable")
}

func decodeStatement(data json.RawMessage) (statement, error) {
	if name, err := scalar[string](data); err == nil && name == "skip" {
		return skipStatement{}, nil
	}
	f, err := object(data)
	if err != nil {
		return nil, err
	}
	if len(f) != 1 {
		return nil, fmt.Errorf("AST: expected one statement kind")
	}
	for kind, payload := range f {
		switch kind {
		case "read":
			name, err := variableName(payload)
			return readStatement{name: name}, err
		case "write":
			value, err := decodeExpression(payload)
			return writeStatement{value: value}, err
		case "set", "assn", "seq", "if", "while", "do":
		default:
			return nil, fmt.Errorf("AST: unknown statement %q", kind)
		}
		p, err := object(payload)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "set":
			if err := p.require("dst", "idx", "value"); err != nil {
				return nil, err
			}
			object, err := decodeExpression(p["dst"])
			if err != nil {
				return nil, err
			}
			index, err := decodeExpression(p["idx"])
			if err != nil {
				return nil, err
			}
			value, err := decodeExpression(p["value"])
			return setStatement{object: object, index: index, value: value}, err
		case "assn":
			if err := p.require("dst", "src"); err != nil {
				return nil, err
			}
			name, err := variableName(p["dst"])
			if err != nil {
				return nil, err
			}
			value, err := decodeExpression(p["src"])
			return assignmentStatement{name: name, value: value}, err
		case "seq":
			if err := p.require("left", "right"); err != nil {
				return nil, err
			}
			left, err := decodeStatement(p["left"])
			if err != nil {
				return nil, err
			}
			right, err := decodeStatement(p["right"])
			return blockStatement{statements: []statement{left, right}}, err
		case "if":
			if err := p.require("cond", "then", "else"); err != nil {
				return nil, err
			}
			condition, err := decodeExpression(p["cond"])
			if err != nil {
				return nil, err
			}
			then, err := decodeStatement(p["then"])
			if err != nil {
				return nil, err
			}
			otherwise, err := decodeStatement(p["else"])
			if _, skip := otherwise.(skipStatement); skip {
				otherwise = nil
			}
			return ifStatement{condition: condition, then: then, otherwise: otherwise}, err
		case "while", "do":
			if err := p.require("cond", "body"); err != nil {
				return nil, err
			}
			condition, err := decodeExpression(p["cond"])
			if err != nil {
				return nil, err
			}
			body, err := decodeStatement(p["body"])
			if kind == "while" {
				return whileStatement{condition: condition, body: body}, err
			}
			return doStatement{condition: condition, body: body}, err
		}
	}
	panic("unreachable")
}
