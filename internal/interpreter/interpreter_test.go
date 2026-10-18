package interpreter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestWebReference(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/interpreter/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                   string
			Program                json.RawMessage
			Input, Output, Message string
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) < 80 {
		t.Fatalf("reference corpus too small: %d", len(corpus.Cases))
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(tc.Program, strings.NewReader(tc.Input), &out)
			if tc.Message != "" {
				if err == nil || err.Error() != tc.Message {
					t.Fatalf("error=%v, want %q", err, tc.Message)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.Output {
				t.Fatalf("output=%q, want %q", out.String(), tc.Output)
			}
		})
	}
}

func node(kind string, payload any) any { return map[string]any{kind: payload} }
func variable(name string) any          { return node("var", name) }
func number(n int32) any                { return node("const", n) }
func write(e any) any                   { return node("write", e) }
func assign(name string, e any) any     { return node("assn", map[string]any{"dst": name, "src": e}) }
func set(object, index, v any) any {
	return node("set", map[string]any{"dst": object, "idx": index, "value": v})
}
func sequence(statements ...any) any {
	if len(statements) == 0 {
		return "skip"
	}
	if len(statements) == 1 {
		return statements[0]
	}
	return node("seq", map[string]any{"left": statements[0], "right": sequence(statements[1:]...)})
}
func operation(op string, left, right any) any {
	return map[string]any{"binop": op, "left": left, "right": right}
}
func runProgram(t *testing.T, program any, input string) (string, error) {
	t.Helper()
	data, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Run(data, strings.NewReader(input), &out)
	return out.String(), err
}

func TestCorrectedInputAndEmptyArrays(t *testing.T) {
	echo := sequence(node("read", "x"), write(variable("x")))
	tests := []struct {
		name, input, want string
		program           any
	}{
		{"string", "\"hello\"", "hello", echo},
		{"one_byte_string", "\"x\"", "x", echo},
		{"empty_string", "\"\"", "", echo},
		{"quote", "\"a\"\"b\"", "a\"b", echo},
		{"comma_and_spaces", "\"a, b; c\"", "a, b; c", echo},
		{"literal_backslash", "\"a\\nb\"", "a\\nb", echo},
		{"multiline", "\"a\nb\"", "a\nb", echo},
		{"utf8", "\"я🙂\"", "я🙂", echo},
		{"empty_array", "[]", "[]", echo},
		{"nested_mixed", "[1,\"x\",[2,[]]]", "[1; x; [2; []]]", echo},
		{"comments", " (* nested (* comment *) *) [1,-- line\n\"--(*text*)\",[]] ", "[1; --(*text*); []]", echo},
		{"empty_input", "", "1", write(number(1))},
		{"comments_only", "-- end", "1", write(number(1))},
		{"empty_ast_array", "", "[]", write(node("array", []any{}))},
		{"empty_array_length", "", "0", write(node("length", node("array", []any{})))},
		{"empty_array_compare", "", "1", write(operation("==", node("array", []any{}), node("array", []any{})))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runProgram(t, tc.program, tc.input)
			if err != nil || out != tc.want {
				t.Fatalf("output=%q error=%v; want %q", out, err, tc.want)
			}
		})
	}
	out, err := runProgram(t, echo, "")
	if out != "" || err == nil || err.Error() != "ERROR: input exhausted" {
		t.Fatalf("empty read: %q %v", out, err)
	}
}

func TestInvalidInput(t *testing.T) {
	for _, input := range []string{"1 2", "1,", ",1", "[1,]", "[1", "[", "\"unterminated", "(*", "(* nested (* *)", "-", "+1", "1.5", "2147483648", "-2147483649", "true", "null", "1;2", "[]garbage", "\"a\"b"} {
		t.Run(input, func(t *testing.T) {
			out, err := runProgram(t, write(number(1)), input)
			if out != "" || err == nil || !strings.HasPrefix(err.Error(), "Input error:") {
				t.Fatalf("output=%q error=%v", out, err)
			}
		})
	}
}

func TestInvalidAST(t *testing.T) {
	for _, program := range []string{
		`null`, `[]`, `{}`, `1`, `"no"`, `{"skip":null}`, `{"read":null}`, `{"read":1}`, `{"write":{"const":null}}`,
		`{"write":{"const":2147483648}}`, `{"write":{"const":1.0}}`, `{"write":{"var":1}}`, `{"write":{"string":null}}`,
		`{"write":{"array":null}}`, `{"write":{"array":[null]}}`, `{"write":{"length":1}}`,
		`{"write":{"elem":{"const":1}}}`, `{"write":{"binop":"+","left":{"const":1}}}`,
		`{"write":{"binop":"?","left":{"const":1},"right":{"const":2}}}`,
		`{"if":{"cond":{"const":1},"then":"skip"}}`, `{"while":{"body":"skip"}}`,
		`{"assn":{"dst":"x","src":{"const":1},"extra":1}}`, `{"set":{"dst":{"var":"x"},"idx":{"const":0}}}`,
		`{"read":"x","write":{"const":1}}`, `{"write":{"unknown":1}}`, `"skip" "skip"`, `{"write":`,
	} {
		t.Run(program, func(t *testing.T) {
			var out bytes.Buffer
			err := Run([]byte(program), strings.NewReader(""), &out)
			if err == nil || !strings.HasPrefix(err.Error(), "AST:") || out.Len() != 0 {
				t.Fatalf("output=%q error=%v", out.String(), err)
			}
		})
	}
	invalid := `{"seq":{"left":{"write":{"const":1}},"right":{"if":{"cond":{"const":0},"then":{"bogus":1},"else":"skip"}}}}`
	var out bytes.Buffer
	if err := Run([]byte(invalid), strings.NewReader(""), &out); err == nil || out.Len() != 0 {
		t.Fatalf("output=%q error=%v", out.String(), err)
	}
}

func TestCyclesAndSharedReferences(t *testing.T) {
	a := variable("a")
	b := variable("b")
	initA := assign("a", node("array", []any{number(0)}))
	cycle := set(a, number(0), a)
	tests := []struct {
		name          string
		program       any
		want, message string
	}{
		{"format_cycle", sequence(initA, write(number(7)), cycle, write(a)), "", "ERROR: cyclic array"},
		{"compare_cycle", sequence(initA, cycle, write(operation("==", a, a))), "", "ERROR: cyclic array"},
		{"independent_cycles", sequence(initA, cycle, assign("b", node("array", []any{number(0)})), set(b, number(0), b), write(operation("==", a, b))), "", "ERROR: cyclic array"},
		{"store_cycle_without_traversal", sequence(initA, cycle, write(node("length", a))), "1", ""},
		{"shared_without_cycle", sequence(initA, assign("b", node("array", []any{a, a})), set(a, number(0), number(3)), write(b), write(operation("==", b, b))), "[[3]; [3]]; 1", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runProgram(t, tc.program, "")
			if tc.message != "" {
				if err == nil || err.Error() != tc.message {
					t.Fatalf("error=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if out != tc.want {
				t.Fatalf("output=%q want=%q", out, tc.want)
			}
		})
	}
}

type brokenReader struct{ err error }

func (r brokenReader) Read([]byte) (int, error) { return 0, r.err }

type brokenWriter struct {
	err   error
	short bool
}

func (w brokenWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, w.err
}
func TestIOErrors(t *testing.T) {
	sentinel := errors.New("device failed")
	var out bytes.Buffer
	if err := Run([]byte(`"skip"`), brokenReader{sentinel}, &out); !errors.Is(err, sentinel) {
		t.Fatalf("read: %v", err)
	}
	program := []byte(`{"write":{"const":1}}`)
	if err := Run(program, strings.NewReader(""), brokenWriter{err: sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("write: %v", err)
	}
	if err := Run(program, strings.NewReader(""), brokenWriter{short: true}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
}

func TestRunIndependence(t *testing.T) {
	program := sequence(node("read", "x"), write(variable("x")))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				out, err := runProgram(t, program, "[1,\"hello\"]")
				if err != nil || out != "[1; hello]" {
					t.Errorf("output=%q error=%v", out, err)
				}
			}
		}()
	}
	wg.Wait()
	out, err := runProgram(t, write(variable("x")), "")
	if out != "" || err == nil || err.Error() != `ERROR: variable "x" is undefined` {
		t.Fatalf("state leaked: %q %v", out, err)
	}
}
