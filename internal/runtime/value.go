package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Value = any

type array struct{ elements []Value }

var errCycle = errors.New("ERROR: cyclic array")

func format(v Value, active map[*array]bool) (string, error) {
	switch v := v.(type) {
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case []byte:
		return string(v), nil
	case *array:
		if active[v] {
			return "", errCycle
		}
		active[v] = true
		defer delete(active, v)
		elements := make([]string, len(v.elements))
		for i, e := range v.elements {
			var err error
			elements[i], err = format(e, active)
			if err != nil {
				return "", err
			}
		}
		return "[" + strings.Join(elements, "; ") + "]", nil
	default:
		return "", fmt.Errorf("runtime: unsupported value %T", v)
	}
}

func formatted(v Value) (string, error) { return format(v, make(map[*array]bool)) }
func Integer(v Value) (int32, error) {
	if n, ok := v.(int32); ok {
		return n, nil
	}
	text, err := formatted(v)
	if err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("ERROR: integer value expected but \"%s\" given", text)
}

func Length(v Value) (int32, error) {
	switch v := v.(type) {
	case []byte:
		return int32(len(v)), nil
	case *array:
		return int32(len(v.elements)), nil
	default:
		text, err := formatted(v)
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("ERROR: array/string value expected but \"%s\" is given", text)
	}
}

func checkIndex(v Value, index int32) error {
	n, err := Length(v)
	if err != nil {
		return err
	}
	if index < 0 || index >= n {
		return fmt.Errorf("ERROR: index %d is out of bounds [0, %d)", index, n)
	}
	return nil
}

type arrayPair struct{ left, right *array }

func compare(op string, left, right Value, active map[arrayPair]bool) (int, error) {
	switch l := left.(type) {
	case int32:
		if r, ok := right.(int32); ok {
			if l < r {
				return -1, nil
			}
			if l > r {
				return 1, nil
			}
			return 0, nil
		}
	case []byte:
		if r, ok := right.([]byte); ok {
			return bytes.Compare(l, r), nil
		}
	case *array:
		if r, ok := right.(*array); ok {
			if len(l.elements) < len(r.elements) {
				return -1, nil
			}
			if len(l.elements) > len(r.elements) {
				return 1, nil
			}
			pair := arrayPair{l, r}
			if active[pair] {
				return 0, errCycle
			}
			active[pair] = true
			defer delete(active, pair)
			for i := range l.elements {
				c, err := compare(op, l.elements[i], r.elements[i], active)
				if err != nil || c != 0 {
					return c, err
				}
			}
			return 0, nil
		}
	}
	l, err := formatted(left)
	if err != nil {
		return 0, err
	}
	r, err := formatted(right)
	if err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("ERROR: binary operator \"%s\" is applied to %s and %s", op, l, r)
}

func ValidOperator(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "!!":
		return true
	}
	return false
}
func Binary(op string, left, right Value) (Value, error) {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		diagnosticOp := op
		if op == "==" {
			diagnosticOp = "="
		}
		if op == "!=" {
			diagnosticOp = "<>"
		}
		c, err := compare(diagnosticOp, left, right, make(map[arrayPair]bool))
		if err != nil {
			return nil, err
		}
		result := false
		switch op {
		case "==":
			result = c == 0
		case "!=":
			result = c != 0
		case "<":
			result = c < 0
		case "<=":
			result = c <= 0
		case ">":
			result = c > 0
		case ">=":
			result = c >= 0
		}
		if result {
			return int32(1), nil
		}
		return int32(0), nil
	}
	r, err := Integer(right)
	if err != nil {
		return nil, err
	}
	l, err := Integer(left)
	if err != nil {
		return nil, err
	}
	switch op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "*":
		return l * r, nil
	case "/", "%":
		if r == 0 {
			return nil, errors.New("Division_by_zero")
		}
		if op == "/" {
			return l / r, nil
		}
		return l % r, nil
	case "&&":
		if l != 0 && r != 0 {
			return int32(1), nil
		}
		return int32(0), nil
	case "!!":
		if l != 0 || r != 0 {
			return int32(1), nil
		}
		return int32(0), nil
	}
	return nil, fmt.Errorf("unknown operator %q", op)
}

func decodeText(raw string) string {
	raw = strings.TrimPrefix(raw, "\ufeff")
	if utf8.ValidString(raw) {
		return raw
	}
	var out strings.Builder
	for len(raw) > 0 {
		r, size := utf8.DecodeRuneInString(raw)
		if r == utf8.RuneError && size == 1 {
			// Replace an incomplete UTF-8 prefix once, as TextDecoder does.
			for size < len(raw) && !utf8.FullRuneInString(raw[:size+1]) {
				size++
			}
		}
		out.WriteRune(r)
		raw = raw[size:]
	}
	return out.String()
}

func NewString(text string) Value { return []byte(text) }

func NewArray(elements []Value) Value {
	return &array{elements: append([]Value(nil), elements...)}
}

func Element(object, index Value) (Value, error) {
	i, err := Integer(index)
	if err != nil {
		return nil, err
	}
	if err := checkIndex(object, i); err != nil {
		return nil, err
	}
	switch object := object.(type) {
	case []byte:
		return int32(object[i]), nil
	case *array:
		return object.elements[i], nil
	default:
		return nil, fmt.Errorf("unsupported value %T", object)
	}
}

func Set(object, index, v Value) error {
	i, err := Integer(index)
	if err != nil {
		return err
	}
	if err := checkIndex(object, i); err != nil {
		return err
	}
	switch object := object.(type) {
	case *array:
		object.elements[i] = v
	case []byte:
		n, err := Integer(v)
		if err != nil {
			return err
		}
		if n < 0 || n > 255 {
			return errors.New(`Invalid_argument("Char.chr")`)
		}
		object[i] = byte(n)
	default:
		return fmt.Errorf("unsupported value %T", object)
	}
	return nil
}

func NormalizeError(err error) error {
	text := decodeText(err.Error())
	if text == err.Error() {
		return err
	}
	return errors.New(text)
}

func WriteOutput(output io.Writer, values []Value) error {
	results := make([]string, len(values))
	for i, v := range values {
		var err error
		results[i], err = formatted(v)
		if err != nil {
			return err
		}
	}
	text := decodeText(strings.Join(results, "; "))
	if text == "" {
		return nil
	}
	n, err := io.WriteString(output, text)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if n != len(text) {
		return fmt.Errorf("write output: %w", io.ErrShortWrite)
	}
	return nil
}
