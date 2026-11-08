package stackmachine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSExpressionStackStorage(t *testing.T) {
	for _, test := range []struct{ name, code, output string }{
		{"preserve_lower_stack", `[{"CONST":99},{"CONST":1},{"CONST":2},{"SEXP":["Pair",2]},"WRITE","WRITE"]`, "Pair (1, 2); 99"},
		{"nullary_preserves_stack", `[{"CONST":7},{"SEXP":["Leaf",0]},"WRITE","WRITE"]`, "Leaf; 7"},
		{"copy_arguments", `[{"CONST":1},{"CONST":2},{"SEXP":["Pair",2]},{"ST":"x"},{"CONST":99},{"CONST":100},{"SEXP":["Pair",2]},"WRITE",{"LD":"x"},"WRITE"]`, "Pair (99, 100); Pair (1, 2)"},
		{"keep_argument_alias", `[{"CONST":1},{"ARRAY":1},{"ST":"a"},{"LD":"a"},{"SEXP":["Box",1]},"WRITE",{"LD":"a"},{"CONST":0},{"CONST":9},"SET"]`, "Box ([9])"},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := newMachine(t, test.code)
			var out bytes.Buffer
			if err := machine.Run(strings.NewReader(""), &out); err != nil || out.String() != test.output {
				t.Fatalf("output=%q, error=%v; want %q", out.String(), err, test.output)
			}
		})
	}
}

func TestSExpressionUnderflow(t *testing.T) {
	for available := 0; available < 3; available++ {
		var instructions []any
		for i := 0; i < available; i++ {
			instructions = append(instructions, map[string]int{"CONST": i})
		}
		instructions = append(instructions, map[string]any{"SEXP": []any{"Triple", 3}})
		code, err := json.Marshal(instructions)
		if err != nil {
			t.Fatal(err)
		}
		machine := newMachine(t, string(code))
		if err := machine.Run(strings.NewReader(""), &bytes.Buffer{}); err != errStack {
			t.Fatalf("%d arguments: %v", available, err)
		}
	}
	machine := newMachine(t, `[{"SEXP":["Huge",2147483647]}]`)
	if err := machine.Run(strings.NewReader(""), &bytes.Buffer{}); err != errStack {
		t.Fatalf("large count: %v", err)
	}
}

func TestInvalidSExpressionInstructions(t *testing.T) {
	for _, code := range []string{
		`["SEXP"]`, `[{"SEXP":null}]`, `[{"SEXP":[]}]`, `[{"SEXP":"Pair"}]`,
		`[{"SEXP":["Pair"]}]`, `[{"SEXP":["Pair",2,3]}]`, `[{"SEXP":{"name":"Pair","count":2}}]`,
		`[{"SEXP":[null,0]}]`, `[{"SEXP":[1,0]}]`, `[{"SEXP":["",0]}]`,
		`[{"SEXP":["Pair",null]}]`, `[{"SEXP":["Pair",-1]}]`, `[{"SEXP":["Pair",1.5]}]`,
		`[{"SEXP":["Pair",2147483648]}]`, `[{"SEXP":["Pair","2"]}]`,
		`[{"SEXP":["Pair",true]}]`, `[{"SEXP":["Pair",{}]}]`,
		`[{"SEXP":["A",0],"SEXP":["B",0]}]`,
	} {
		t.Run(code, func(t *testing.T) {
			if _, err := NewStackMachine([]byte(code)); err == nil {
				t.Fatalf("accepted invalid instruction: %s", code)
			}
		})
	}
}
