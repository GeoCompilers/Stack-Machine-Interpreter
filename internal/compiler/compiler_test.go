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
