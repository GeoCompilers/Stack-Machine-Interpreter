package interpreter

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func decodeExpression(data json.RawMessage) (*expression, error) {
	f, err := object(data)
	if err != nil {
		return nil, err
	}
	e := &expression{}
	switch {
	case f["binop"] != nil:
		if err = f.require("binop", "left", "right"); err != nil {
			break
		}
		e.kind = "binop"
		e.text, err = scalar[string](f["binop"])
		if err == nil && !validOperator(e.text) {
			err = fmt.Errorf("AST: unknown operator %q", e.text)
		}
		if err == nil {
			e.left, err = decodeExpression(f["left"])
		}
		if err == nil {
			e.right, err = decodeExpression(f["right"])
		}
	case f["elem"] != nil:
		if err = f.require("elem", "index"); err != nil {
			break
		}
		e.kind = "elem"
		e.left, err = decodeExpression(f["elem"])
		if err == nil {
			e.right, err = decodeExpression(f["index"])
		}
	default:
		if len(f) != 1 {
			return nil, fmt.Errorf("AST: expected one expression kind")
		}
		for kind, payload := range f {
			e.kind = kind
			switch kind {
			case "const":
				e.number, err = scalar[int32](payload)
			case "var", "string":
				e.text, err = scalar[string](payload)
			case "length":
				e.left, err = decodeExpression(payload)
			case "array":
				var raw []json.RawMessage
				raw, err = scalar[[]json.RawMessage](payload)
				for _, item := range raw {
					if err != nil {
						break
					}
					var element *expression
					element, err = decodeExpression(item)
					e.elements = append(e.elements, element)
				}
			default:
				err = fmt.Errorf("AST: unknown expression %q", kind)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func decodeStatement(data json.RawMessage) (*statement, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte(`"skip"`)) {
		return &statement{kind: "skip"}, nil
	}
	if v, err := scalar[string](data); err == nil && v == "skip" {
		return &statement{kind: "skip"}, nil
	}
	f, err := object(data)
	if err != nil {
		return nil, err
	}
	if len(f) != 1 {
		return nil, fmt.Errorf("AST: expected one statement kind")
	}
	s := &statement{}
	for kind, payload := range f {
		s.kind = kind
		switch kind {
		case "read":
			s.name, err = scalar[string](payload)
		case "write":
			s.value, err = decodeExpression(payload)
		case "assn", "set", "seq", "if", "while", "do":
			var p fields
			p, err = object(payload)
			if err != nil {
				break
			}
			switch kind {
			case "assn":
				err = p.require("dst", "src")
				if err == nil {
					s.name, err = scalar[string](p["dst"])
				}
				if err == nil {
					s.value, err = decodeExpression(p["src"])
				}
			case "set":
				err = p.require("dst", "idx", "value")
				if err == nil {
					s.object, err = decodeExpression(p["dst"])
				}
				if err == nil {
					s.index, err = decodeExpression(p["idx"])
				}
				if err == nil {
					s.value, err = decodeExpression(p["value"])
				}
			case "seq":
				err = p.require("left", "right")
				if err == nil {
					s.first, err = decodeStatement(p["left"])
				}
				if err == nil {
					s.second, err = decodeStatement(p["right"])
				}
			case "if":
				err = p.require("cond", "then", "else")
				if err == nil {
					s.condition, err = decodeExpression(p["cond"])
				}
				if err == nil {
					s.first, err = decodeStatement(p["then"])
				}
				if err == nil {
					s.otherwise, err = decodeStatement(p["else"])
				}
			case "while", "do":
				err = p.require("cond", "body")
				if err == nil {
					s.condition, err = decodeExpression(p["cond"])
				}
				if err == nil {
					s.first, err = decodeStatement(p["body"])
				}
			}
		default:
			err = fmt.Errorf("AST: unknown statement %q", kind)
		}
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}
