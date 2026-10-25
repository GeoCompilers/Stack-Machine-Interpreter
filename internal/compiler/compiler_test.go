package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/compiler"
	"stack-machine-interpreter/internal/interpreter"
	"stack-machine-interpreter/internal/stackmachine"
)

type astCase struct {
	Name         string          `json:"name"`
	Program      json.RawMessage `json:"program"`
	Instructions json.RawMessage `json:"instructions"`
	Input        string          `json:"input"`
	Output       string          `json:"output"`
	Message      string          `json:"message"`
	Group        string          `json:"group"`
	Counts       map[string]int  `json:"counts"`
	SharedExit   bool            `json:"sharedExit"`
}

func loadCases(t *testing.T, filename string) []astCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", filename))
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []astCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("empty AST corpus")
	}
	return corpus.Cases
}

func continuationCases(t *testing.T, group string) []astCase {
	t.Helper()
	var selected []astCase
	for _, c := range loadCases(t, "compiler_ast.json") {
		if c.Group == group {
			selected = append(selected, c)
		}
	}
	if len(selected) == 0 {
		t.Fatal("missing continuation cases")
	}
	return selected
}

func TestCompileReferenceASTCases(t *testing.T) {
	changes, err := os.ReadFile("../../tests/testdata/interpreter/sm_expectations.json")
	if err != nil {
		t.Fatal(err)
	}
	var expectations map[string]string
	if err := json.Unmarshal(changes, &expectations); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, test := range loadCases(t, "interpreter/reference.json") {
		count++
		if message, ok := expectations[test.Name]; ok {
			test.Message = message
		}
		t.Run(test.Name, func(t *testing.T) {
			program, err := compiler.Compile(test.Program)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			machine, err := stackmachine.NewStackMachine(program)
			if err != nil {
				t.Fatalf("invalid instructions: %v\n%s", err, program)
			}
			var stdout, direct bytes.Buffer
			err = machine.Run(strings.NewReader(test.Input), &stdout)
			directErr := interpreter.Run(test.Program, strings.NewReader(test.Input), &direct)
			if test.Message != "" {
				if err == nil || err.Error() != test.Message || directErr == nil || directErr.Error() != test.Message {
					t.Fatalf("SM=%v, interpreter=%v, want=%q", err, directErr, test.Message)
				}
				if stdout.Len() != 0 || direct.Len() != 0 {
					t.Fatal("output on failure")
				}
				return
			}
			if err != nil || directErr != nil {
				t.Fatalf("SM=%v, interpreter=%v", err, directErr)
			}
			want := test.Output
			if stdout.String() != want || direct.String() != test.Output {
				t.Fatalf("SM=%q, interpreter=%q, want=%q", stdout.String(), direct.String(), test.Output)
			}
		})
	}
	if count != 127 {
		t.Fatalf("tested %d reference ASTs, want 127", count)
	}
}

func TestCompileRejectsInvalidAST(t *testing.T) {
	cases := []string{
		"", "null", "{}", "[]", `"unknown"`, `{write(1);}`, `"skip" {}`,
		`{"read":null}`, `{"read":1}`, `{"read":""}`, `{"write":{"var":""}}`,
		`{"write":{"const":null}}`, `{"write":{"const":2147483648}}`,
		`{"write":{"const":1.5}}`, `{"write":{"unknown":1}}`,
		`{"write":{"binop":"?","left":{"const":1},"right":{"const":2}}}`,
		`{"write":{"binop":"+","left":{"const":1}}}`,
		`{"write":{"binop":"+","left":{"const":1},"right":{"const":2},"extra":0}}`,
		`{"seq":{"left":"skip"}}`, `{"seq":{"left":"skip","right":null}}`,
		`{"assn":{"src":{"const":1}}}`, `{"assn":{"dst":"x","src":null}}`,
		`{"if":{"cond":{"const":1},"then":"skip"}}`,
		`{"if":{"cond":{"const":1},"then":"skip","else":{"read":null}}}`,
		`{"while":{"cond":{"const":1}}}`, `{"do":{"cond":null,"body":"skip"}}`,
		`{"read":"x","write":{"const":1}}`, `{"for":{}}`,
		`{"write":{"string":null}}`, `{"write":{"array":null}}`, `{"write":{"array":[null]}}`,
		`{"write":{"length":null}}`, `{"write":{"elem":{"const":0}}}`,
		`{"write":{"elem":{"const":0},"index":null}}`,
		`{"set":{"dst":{"var":"a"},"idx":{"const":0}}}`,
		`{"set":{"dst":null,"idx":{"const":0},"value":{"const":1}}}`,
		`{"set":{"dst":{"var":"a"},"idx":null,"value":{"const":1}}}`,
		`{"set":{"dst":{"var":"a"},"idx":{"const":0},"value":null}}`,
	}
	for _, ast := range cases {
		t.Run(ast, func(t *testing.T) {
			program, err := compiler.Compile([]byte(ast))
			if err == nil || program != nil {
				t.Fatalf("expected error and no instructions, got %s, %v", program, err)
			}
		})
	}
}

func TestCompileContinuationInstructions(t *testing.T) {
	cases := continuationCases(t, "instructions")
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			program, err := compiler.Compile(test.Program)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stackmachine.NewStackMachine(program); err != nil {
				t.Fatalf("invalid labels or instructions: %v\n%s", err, program)
			}
			var instructions []any
			if err := json.Unmarshal(program, &instructions); err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			labels := map[string]int{}
			targets := map[string]bool{}
			var jumps []string
			for index, instruction := range instructions {
				if operands, ok := instruction.(map[string]any); ok {
					for opcode, operand := range operands {
						counts[opcode]++
						switch opcode {
						case "LABEL":
							labels[operand.(string)] = index
						case "JMP", "JZ", "JNZ":
							targets[operand.(string)] = true
							if opcode == "JMP" {
								jumps = append(jumps, operand.(string))
							}
						}
					}
				}
			}
			for opcode, want := range test.Counts {
				if counts[opcode] != want {
					t.Errorf("%s count=%d, want %d\n%s", opcode, counts[opcode], want, program)
				}
			}
			for label, index := range labels {
				if !targets[label] {
					t.Errorf("unused label %q", label)
				}
				for next := index + 1; next < len(instructions); next++ {
					operands, ok := instructions[next].(map[string]any)
					if ok && operands["LABEL"] != nil {
						continue
					}
					if ok && operands["JMP"] != nil {
						t.Errorf("jump to %q leads to another unconditional jump\n%s", label, program)
					}
					break
				}
			}
			if test.SharedExit {
				end, ok := instructions[len(instructions)-1].(map[string]any)
				if !ok || end["LABEL"] == nil {
					t.Fatalf("missing program exit label\n%s", program)
				}
				for _, target := range jumps {
					if target != end["LABEL"] {
						t.Errorf("jump target=%q, want shared exit %q", target, end["LABEL"])
					}
				}
			}
		})
	}
}

func TestCompileContinuationExecution(t *testing.T) {
	cases := continuationCases(t, "execution")
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			program, err := compiler.Compile(test.Program)
			if err != nil {
				t.Fatal(err)
			}
			machine, err := stackmachine.NewStackMachine(program)
			if err != nil {
				t.Fatalf("invalid program: %v\n%s", err, program)
			}
			var stdout bytes.Buffer
			if err := machine.Run(strings.NewReader(test.Input), &stdout); err != nil {
				t.Fatalf("run: %v", err)
			}
			if stdout.String() != strings.ReplaceAll(test.Output, ";", "; ") {
				t.Fatalf("output=%q, want %q", stdout.String(), test.Output)
			}
		})
	}
}

func TestCurrentSMReference(t *testing.T) {
	cases := loadCases(t, "sm_reference.json")
	if len(cases) != 149 {
		t.Fatalf("expected 149 reference cases, got %d", len(cases))
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			compiled, err := compiler.Compile(test.Program)
			if err != nil {
				t.Fatal(err)
			}
			for _, code := range []struct {
				name string
				data []byte
			}{{"compiled", compiled}, {"reference", test.Instructions}} {
				t.Run(code.name, func(t *testing.T) {
					machine, err := stackmachine.NewStackMachine(code.data)
					if err != nil {
						t.Fatal(err)
					}
					var out bytes.Buffer
					err = machine.Run(strings.NewReader(test.Input), &out)
					if test.Message == "" && err != nil || test.Message != "" && (err == nil || err.Error() != test.Message) {
						t.Fatalf("error=%v, want %q", err, test.Message)
					}
					if out.String() != test.Output {
						t.Fatalf("output=%q, want %q", out.String(), test.Output)
					}
				})
			}
			var out bytes.Buffer
			err = interpreter.Run(test.Program, strings.NewReader(test.Input), &out)
			if test.Message == "" && err != nil || test.Message != "" && (err == nil || err.Error() != test.Message) {
				t.Fatalf("AST error=%v, want %q", err, test.Message)
			}
			if out.String() != test.Output {
				t.Fatalf("AST output=%q, want %q", out.String(), test.Output)
			}
		})
	}
}

func TestCompileAggregateInstructions(t *testing.T) {
	for _, test := range []struct{ ast, code string }{
		{`{"write":{"array":[{"const":1},{"string":"x"},{"array":[]}]}}`, `[{"CONST":1},{"STRING":"x"},{"ARRAY":0},{"ARRAY":3},"WRITE"]`},
		{`{"write":{"elem":{"var":"a"},"index":{"const":2}}}`, `[{"LD":"a"},{"CONST":2},"ELEM","WRITE"]`},
		{`{"set":{"dst":{"var":"a"},"idx":{"const":2},"value":{"string":""}}}`, `[{"LD":"a"},{"CONST":2},{"STRING":""},"SET"]`},
		{`{"write":{"length":{"string":"я"}}}`, `[{"STRING":"я"},"LENGTH","WRITE"]`},
	} {
		code, err := compiler.Compile([]byte(test.ast))
		if err != nil {
			t.Fatal(err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, code); err != nil {
			t.Fatal(err)
		}
		if compact.String() != test.code {
			t.Fatalf("code=%s, want %s", compact.String(), test.code)
		}
	}
}
