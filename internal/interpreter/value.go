package interpreter

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type value = any

type array struct{ elements []value }

var errCycle = errors.New("ERROR: cyclic array")

func format(v value, active map[*array]bool) (string, error) {
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
		return "", fmt.Errorf("interpreter: unsupported value %T", v)
	}
}

func formatted(v value) (string, error) { return format(v, make(map[*array]bool)) }
func asInteger(v value) (int32, error) {
	if n, ok := v.(int32); ok {
		return n, nil
	}
	text, err := formatted(v)
	if err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("ERROR: integer value expected but \"%s\" given", text)
}

func length(v value, setting bool) (int, error) {
	switch v := v.(type) {
	case []byte:
		return len(v), nil
	case *array:
		return len(v.elements), nil
	default:
		text, err := formatted(v)
		if err != nil {
			return 0, err
		}
		extra := ""
		if setting {
			extra = "is "
		}
		return 0, fmt.Errorf("ERROR: array/string value expected but \"%s\" %sgiven", text, extra)
	}
}

func checkIndex(v value, index int32, setting bool) error {
	n, err := length(v, setting)
	if err != nil {
		return err
	}
	if index < 0 || int64(index) >= int64(n) {
		return fmt.Errorf("ERROR: index %d is out of bounds [0, %d)", index, n)
	}
	return nil
}

type arrayPair struct{ left, right *array }

func compare(op string, left, right value, active map[arrayPair]bool) (int, error) {
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

func validOperator(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "!!":
		return true
	}
	return false
}
func binary(op string, left, right value) (value, error) {
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
	r, err := asInteger(right)
	if err != nil {
		return nil, err
	}
	l, err := asInteger(left)
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
	return nil, fmt.Errorf("AST: unknown operator %q", op)
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
