package interpreter

import "fmt"

func (m *machine) eval(e *expression) (value, error) {
	switch e.kind {
	case "const":
		return e.number, nil
	case "string":
		return []byte(e.text), nil
	case "var":
		v, ok := m.variables[e.text]
		if !ok {
			return nil, fmt.Errorf("ERROR: variable \"%s\" is undefined", e.text)
		}
		return v, nil
	case "array":
		elements := make([]value, len(e.elements))
		for i, item := range e.elements {
			v, err := m.eval(item)
			if err != nil {
				return nil, err
			}
			elements[i] = v
		}
		return &array{elements: elements}, nil
	case "binop":
		right, err := m.eval(e.right)
		if err != nil {
			return nil, err
		}
		left, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		return binary(e.text, left, right)
	case "length":
		object, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		n, err := length(object, false)
		return int32(n), err
	case "elem":
		index, err := m.eval(e.right)
		if err != nil {
			return nil, err
		}
		object, err := m.eval(e.left)
		if err != nil {
			return nil, err
		}
		i, err := asInteger(index)
		if err != nil {
			return nil, err
		}
		if err = checkIndex(object, i, false); err != nil {
			return nil, err
		}
		switch object := object.(type) {
		case []byte:
			return int32(object[i]), nil
		case *array:
			return object.elements[i], nil
		default:
			return nil, fmt.Errorf("interpreter: unsupported value %T", object)
		}
	}
	return nil, fmt.Errorf("AST: unknown expression %q", e.kind)
}
