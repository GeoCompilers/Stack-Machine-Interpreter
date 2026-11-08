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
type sexpression struct {
	name string
	args []Value
}

var errCycle = errors.New("ERROR: cyclic array")
var errSExpressionCycle = errors.New("ERROR: cyclic S-expression")

func format(v Value, active map[Value]bool) (string, error) {
	switch v := v.(type) {
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case []byte:
		return string(v), nil
	case *sexpression:
		if active[v] {
			return "", errSExpressionCycle
		}
		active[v] = true
		defer delete(active, v)
		if text, proper, err := formatList(v, active); proper || err != nil {
			return text, err
		}
		if len(v.args) == 0 {
			return v.name, nil
		}
		args := make([]string, len(v.args))
		for i, arg := range v.args {
			var err error
			args[i], err = format(arg, active)
			if err != nil {
				return "", err
			}
		}
		return v.name + " (" + strings.Join(args, ", ") + ")", nil
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

// Walk list tails iteratively. Only ancestors are active, so shared tails and
// repeated subexpressions remain legal while actual cycles are rejected.
func formatList(v *sexpression, active map[Value]bool) (string, bool, error) {
	var parts []string
	var tails []*sexpression
	defer func() {
		for _, tail := range tails {
			delete(active, tail)
		}
	}()
	for {
		if v.name == "nil" && len(v.args) == 0 {
			return "{" + strings.Join(parts, ", ") + "}", true, nil
		}
		if v.name != "cons" || len(v.args) != 2 {
			return "", false, nil
		}
		head, err := format(v.args[0], active)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, head)
		tail, ok := v.args[1].(*sexpression)
		if !ok {
			return "", false, nil
		}
		if active[tail] {
			return "", false, errSExpressionCycle
		}
		active[tail] = true
		tails = append(tails, tail)
		v = tail
	}
}

func formatted(v Value) (string, error) { return format(v, make(map[Value]bool)) }
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
	case *sexpression:
		return int32(len(v.args)), nil
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

type valuePair struct{ left, right Value }

func compare(op string, left, right Value, active map[valuePair]bool) (int, error) {
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
	case *sexpression:
		if r, ok := right.(*sexpression); ok {
			if c := strings.Compare(l.name, r.name); c != 0 {
				return c, nil
			}
			if len(l.args) < len(r.args) {
				return -1, nil
			}
			if len(l.args) > len(r.args) {
				return 1, nil
			}
			pair := valuePair{l, r}
			if active[pair] {
				return 0, errSExpressionCycle
			}
			active[pair] = true
			defer delete(active, pair)
			for i := range l.args {
				c, err := compare(op, l.args[i], r.args[i], active)
				if err != nil || c != 0 {
					return c, err
				}
			}
			return 0, nil
		}
	case *array:
		if r, ok := right.(*array); ok {
			if len(l.elements) < len(r.elements) {
				return -1, nil
			}
			if len(l.elements) > len(r.elements) {
				return 1, nil
			}
			pair := valuePair{l, r}
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
		c, err := compare(diagnosticOp, left, right, make(map[valuePair]bool))
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

func NewSExpression(name string, args []Value) Value {
	return &sexpression{name: name, args: append([]Value(nil), args...)}
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
	case *sexpression:
		return object.args[i], nil
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
	case *sexpression:
		object.args[i] = v
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
