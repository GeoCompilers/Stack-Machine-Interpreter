package compiler_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/compiler"
	"stack-machine-interpreter/internal/interpreter"
	"stack-machine-interpreter/internal/stackmachine"
)

func TestSExpressionReference(t *testing.T) {
	for _, test := range loadCases(t, "sexp_reference.json") {
		t.Run(test.Name, func(t *testing.T) {
			compiled, err := compiler.Compile(test.Program)
			if err != nil {
				t.Fatal(err)
			}
			routes := map[string]func(io.Reader, io.Writer) error{
				"AST": func(input io.Reader, output io.Writer) error {
					return interpreter.Run(test.Program, input, output)
				},
			}
			for name, code := range map[string][]byte{"compiled": compiled, "reference": test.Instructions} {
				machine, err := stackmachine.NewStackMachine(code)
				if err != nil {
					t.Fatal(err)
				}
				routes[name] = machine.Run
			}
			for name, run := range routes {
				t.Run(name, func(t *testing.T) {
					// Reusing a machine must not reuse mutated constructor values.
					for i := 0; i < 2; i++ {
						var out bytes.Buffer
						err := run(strings.NewReader(test.Input), &out)
						message := ""
						if err != nil {
							message = err.Error()
						}
						if message != test.Message || out.String() != test.Output {
							t.Fatalf("output=%q, error=%q; want %q, %q", out.String(), message, test.Output, test.Message)
						}
					}
				})
			}
		})
	}
}

func TestInvalidSExpressionAST(t *testing.T) {
	for _, expression := range []string{
		`{"sexp":"Pair"}`, `{"sexp":null,"args":[]}`, `{"sexp":1,"args":[]}`,
		`{"sexp":"","args":[]}`, `{"sexp":"Pair","args":null}`,
		`{"sexp":"Pair","args":{}}`, `{"sexp":"Pair","args":[null]}`,
		`{"sexp":"Pair","args":[{"const":1},{}]}`,
		`{"sexp":"Pair","args":[],"extra":1}`, `{"args":[]}`,
	} {
		t.Run(expression, func(t *testing.T) {
			ast := []byte(`{"write":` + expression + `}`)
			code, err := compiler.Compile(ast)
			if err == nil || code != nil {
				t.Fatalf("compiler accepted invalid AST: %s", ast)
			}
			var out bytes.Buffer
			if err := interpreter.Run(ast, strings.NewReader(""), &out); err == nil || out.Len() != 0 {
				t.Fatalf("interpreter accepted invalid AST: %s", ast)
			}
		})
	}
}
