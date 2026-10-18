package interpreter

import (
	"errors"
	"fmt"
)

func (m *machine) exec(s *statement) error {
	for {
		switch s.kind {
		case "skip":
			return nil
		case "assn":
			v, err := m.eval(s.value)
			if err != nil {
				return err
			}
			m.variables[s.name] = v
			return nil
		case "read":
			if m.inputIndex == len(m.input) {
				return errors.New("ERROR: input exhausted")
			}
			m.variables[s.name] = m.input[m.inputIndex]
			m.inputIndex++
			return nil
		case "write":
			v, err := m.eval(s.value)
			if err != nil {
				return err
			}
			m.output = append(m.output, v)
			return nil
		case "seq":
			if err := m.exec(s.first); err != nil {
				return err
			}
			s = s.second
		case "if":
			yes, err := m.condition(s.condition)
			if err != nil {
				return err
			}
			if yes {
				s = s.first
			} else {
				s = s.otherwise
			}
		case "while", "do":
			first := s.kind == "do"
			for {
				if !first {
					yes, err := m.condition(s.condition)
					if err != nil {
						return err
					}
					if !yes {
						return nil
					}
				}
				first = false
				if err := m.exec(s.first); err != nil {
					return err
				}
			}
		case "set":
			return m.set(s)
		default:
			return fmt.Errorf("AST: unknown statement %q", s.kind)
		}
	}
}

func (m *machine) condition(e *expression) (bool, error) {
	v, err := m.eval(e)
	if err != nil {
		return false, err
	}
	n, err := asInteger(v)
	return n != 0, err
}

func (m *machine) set(s *statement) error {
	v, err := m.eval(s.value)
	if err != nil {
		return err
	}
	index, err := m.eval(s.index)
	if err != nil {
		return err
	}
	object, err := m.eval(s.object)
	if err != nil {
		return err
	}
	i, err := asInteger(index)
	if err != nil {
		return err
	}
	if err = checkIndex(object, i, true); err != nil {
		return err
	}
	switch object := object.(type) {
	case *array:
		object.elements[i] = v
		return nil
	case []byte:
		n, err := asInteger(v)
		if err != nil {
			return err
		}
		if n < 0 || n > 255 {
			return errors.New(`Invalid_argument("Char.chr")`)
		}
		object[i] = byte(n)
		return nil
	default:
		return fmt.Errorf("interpreter: unsupported value %T", object)
	}
}
