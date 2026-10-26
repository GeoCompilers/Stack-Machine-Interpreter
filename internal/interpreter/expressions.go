package interpreter

import (
	"fmt"
	"stack-machine-interpreter/internal/runtime"
)

func (m *machine) eval(e *expression) (runtime.Value, error) {
	switch e.kind {
	case "const":
		return e.number, nil
	case "string":
		return runtime.NewString(e.text), nil
	case "var":
		v, ok := m.variables[e.text]
		if !ok {
			return nil, fmt.Errorf("ERROR: variable \"%s\" is undefined", e.text)
		}
		return v, nil
	case "array":
		elements := make([]runtime.Value, len(e.elements))
		for i, item := range e.elements {
			v, err := m.eval(item)
			if err != nil {
				return nil, err
			}
			elements[i] = v
		}
		return runtime.NewArray(elements), nil
	case "binop":
		left, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		right, err := m.eval(e.right)
		if err != nil {
			return nil, err
		}
		return runtime.Binary(e.text, left, right)
	case "length":
		object, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		return runtime.Length(object)
	case "elem":
		object, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		index, err := m.eval(e.right)
		if err != nil {
			return nil, err
		}
		return runtime.Element(object, index)
	}
	return nil, fmt.Errorf("AST: unknown expression %q", e.kind)
}
