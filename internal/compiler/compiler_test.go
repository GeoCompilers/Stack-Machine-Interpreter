package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/compiler"
	"stack-machine-interpreter/internal/stackmachine"
)

type sourceCase struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Input  string `json:"input"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

func TestCompileReferenceSourceCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []sourceCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			program, err := compiler.Compile(test.Source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			machine, err := stackmachine.NewStackMachine(program)
			if err != nil {
				t.Fatalf("generated invalid program: %v\n%s", err, program)
			}
			input := parseInput(t, test.Input)
			var stdout, stderr bytes.Buffer
			err = machine.Run(input, &stdout, &stderr)
			if test.Error != "" {
				causes := map[string]string{
					"undefined_variable": "unknown variable",
					"input_exhausted":    "input exhausted",
					"division_by_zero":   "division by zero",
				}
				cause, ok := causes[test.Error]
				if !ok {
					t.Fatalf("unknown error category %q", test.Error)
				}
				if err == nil || !strings.Contains(err.Error(), cause) {
					t.Fatalf("expected runtime error %q; got %v; output=%q", cause, err, stdout.String())
				}
				if stderr.String() != err.Error()+"\n" {
					t.Fatalf("diagnostic=%q, want %q", stderr.String(), err.Error()+"\n")
				}
				return
			}
			if err != nil {
				t.Fatalf("run: %v; diagnostic=%q", err, stderr.String())
			}
			if stdout.String() != test.Output {
				t.Fatalf("output=%q, want %q", stdout.String(), test.Output)
			}
		})
	}
}

func TestCompileRejectsInvalidSource(t *testing.T) {
	cases := []string{
		"{}",
		"{write(1)",
		"{x=2147483648;}",
		"{(* unclosed }",
		"{if(1) else skip;}",
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			if _, err := compiler.Compile(source); err == nil {
				t.Fatal("expected compile error")
			}
		})
	}
}

type syntaxCase struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Valid  bool   `json:"valid"`
	Output string `json:"output"`
}

func TestCompileSyntaxCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "testdata", "compiler_syntax.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []syntaxCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			program, err := compiler.Compile(test.Source)
			if !test.Valid {
				if err == nil || program != nil {
					t.Fatalf("expected compile error and no program; err=%v program=%s", err, program)
				}
				return
			}
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			machine, err := stackmachine.NewStackMachine(program)
			if err != nil {
				t.Fatalf("generated invalid program: %v\n%s", err, program)
			}
			var stdout, stderr bytes.Buffer
			if err := machine.Run([]int32{0}, &stdout, &stderr); err != nil {
				t.Fatalf("run: %v; diagnostic=%q", err, stderr.String())
			}
			if stdout.String() != test.Output || stderr.Len() != 0 {
				t.Fatalf("output=%q, want %q; diagnostic=%q", stdout.String(), test.Output, stderr.String())
			}
		})
	}
}

func TestComparisonErrorLocation(t *testing.T) {
	program, err := compiler.Compile("{\nwrite(1 < 2 == 1);\n}")
	if err == nil || program != nil || !strings.Contains(err.Error(), "line 2, column 13") || !strings.Contains(err.Error(), "parentheses") {
		t.Fatalf("expected comparison diagnostic at second operator and no program; err=%v program=%s", err, program)
	}
}

func TestCompileContinuationInstructions(t *testing.T) {
	cases := []struct {
		name, source         string
		jz, jmp, jnz, labels int
		sharedExit           bool
	}{
		{"plain_sequence", "{x=1; write(x); skip;}", 0, 0, 0, 0, false},
		{"if_without_else", "{if(1)write(1);}", 1, 0, 0, 1, true},
		{"nested_without_else", "{if(1)if(0)write(1);}", 2, 0, 0, 1, true},
		{"nested_then", "{if(1){if(0)write(1); else write(2);} else write(3);}", 2, 2, 0, 3, true},
		{"nested_else", "{if(1)write(1); else {if(0)write(2); else write(3);}}", 2, 2, 0, 3, true},
		{"elif", "{if(0)write(1); elif(0)write(2); elif(1)write(3); else write(4);}", 3, 3, 0, 4, true},
		{"adjacent_if", "{if(1)write(1); if(0)write(2);}", 2, 0, 0, 2, false},
		{"nonfinal_nested_if", "{if(1){if(0)write(1); else write(2); write(3);} else write(4); write(5);}", 2, 2, 0, 4, false},
		{"while", "{while(0)skip;}", 0, 1, 1, 2, false},
		{"do_unused_condition_label", "{do skip; while(0);}", 0, 0, 1, 1, false},
		{"do_used_condition_label", "{do if(0)skip; while(0);}", 1, 0, 1, 2, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			program, err := compiler.Compile(test.source)
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
			for opcode, want := range map[string]int{"JZ": test.jz, "JMP": test.jmp, "JNZ": test.jnz, "LABEL": test.labels} {
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
			if test.sharedExit {
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
	cases := []struct {
		name, source, input, output string
	}{
		{"nested_then_true", "{read(a); read(b); if(a){if(b)write(1); else write(2);} else write(3); write(4);}", "1,1", "1;4"},
		{"nested_then_false", "{read(a); read(b); if(a){if(b)write(1); else write(2);} else write(3); write(4);}", "1,0", "2;4"},
		{"outer_false", "{read(a); read(b); if(a){if(b)write(1); else write(2);} else write(3); write(4);}", "0,1", "3;4"},
		{"nested_else_true", "{read(a); read(b); if(a)write(1); else {if(b)write(2); else write(3);} write(4);}", "0,1", "2;4"},
		{"nested_else_false", "{read(a); read(b); if(a)write(1); else {if(b)write(2); else write(3);} write(4);}", "0,0", "3;4"},
		{"nested_no_else_true", "{read(x); if(1){if(x)write(1);} else write(2); write(3);}", "1", "1;3"},
		{"nested_no_else_false", "{read(x); if(1){if(x)write(1);} else write(2); write(3);}", "0", "3"},
		{"nonfinal_nested_if_true", "{read(x); if(1){if(x)write(1); else write(2); write(3);} else write(4); write(5);}", "1", "1;3;5"},
		{"nonfinal_nested_if_false", "{read(x); if(1){if(x)write(1); else write(2); write(3);} else write(4); write(5);}", "0", "2;3;5"},
		{"adjacent_true_false", "{read(x); if(x)write(1); else write(2); if(x==0)write(3); else write(4); write(5);}", "1", "1;4;5"},
		{"adjacent_false_true", "{read(x); if(x)write(1); else write(2); if(x==0)write(3); else write(4); write(5);}", "0", "2;3;5"},
		{"elif_nested", "{read(x); if(x==0)write(0); elif(x==1){if(0)write(99); else write(1);} else write(2); write(3);}", "1", "1;3"},
		{"while_zero", "{i=0; while(i<0)if(1)i+=1; write(i);}", "", "0"},
		{"while_once", "{i=0; while(i<1)if(1)i+=1; write(i);}", "", "1"},
		{"while_nested_if", "{i=0; while(i<3){i+=1; if(i==1){if(0)write(99); else write(i);} else write(i*10);} write(i);}", "", "1;20;30;3"},
		{"do_once", "{i=0; do if(1)i+=1; while(0); write(i);}", "", "1"},
		{"do_nested_if", "{i=0; do {i+=1; if(i==1){if(0)write(99); else write(i);} else write(i*10);} while(i<3); write(i);}", "", "1;20;30;3"},
		{"loop_inside_if", "{if(1){i=0; while(i<2)if(1)i+=1;} else write(99); write(i);}", "", "2"},
		{"for_zero", "{for(if(1)i=3; else i=0; i<3; i+=1)write(99); write(i);}", "", "3"},
		{"for_conditional_clauses", "{for(if(1){if(0)i=99; else i=0;} else i=99; i<3; if(i==0)i+=1; else {if(1)i+=1; else i+=99;}){if(i==1){if(0)write(99); else write(i);} else write(i);} write(i);}", "", "0;1;2;3"},
		{"for_body_before_post", "{for(i=0; i<3; i+=1)if(0)write(99); else write(i); write(i);}", "", "0;1;2;3"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			program, err := compiler.Compile(test.source)
			if err != nil {
				t.Fatal(err)
			}
			machine, err := stackmachine.NewStackMachine(program)
			if err != nil {
				t.Fatalf("invalid program: %v\n%s", err, program)
			}
			var stdout, stderr bytes.Buffer
			if err := machine.Run(parseInput(t, test.input), &stdout, &stderr); err != nil {
				t.Fatalf("run: %v; diagnostic=%q", err, stderr.String())
			}
			if stdout.String() != test.output || stderr.Len() != 0 {
				t.Fatalf("output=%q, want %q; diagnostic=%q", stdout.String(), test.output, stderr.String())
			}
		})
	}
}

func parseInput(t *testing.T, source string) []int32 {
	t.Helper()
	if strings.TrimSpace(source) == "" {
		return nil
	}
	items := strings.Split(source, ",")
	input := make([]int32, len(items))
	for index, item := range items {
		value, err := strconv.ParseInt(strings.TrimSpace(item), 10, 32)
		if err != nil {
			t.Fatalf("parse input item %d: %v", index+1, err)
		}
		input[index] = int32(value)
	}
	return input
}
