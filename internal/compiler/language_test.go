package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/compiler"
	"stack-machine-interpreter/internal/stackmachine"
)

func TestSharedLanguageReference(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/language_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name            string
			Program         json.RawMessage
			Instructions    json.RawMessage
			Input           string
			InterpreterOnly bool
			Expected        struct{ Output, Message string }
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	shared, switches := 0, 0
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			code, err := compiler.Compile(tc.Program)
			if tc.InterpreterOnly {
				switches++
				if err == nil || code != nil || !strings.Contains(err.Error(), "switch is supported only by the AST interpreter") {
					t.Fatalf("compiler accepted switch: %s / %v", code, err)
				}
				return
			}
			shared++
			if err != nil {
				t.Fatal(err)
			}
			for name, instructions := range map[string][]byte{"compiled": code, "reference": tc.Instructions} {
				t.Run(name, func(t *testing.T) {
					machine, err := stackmachine.NewStackMachine(instructions)
					if err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 2; i++ {
						var out bytes.Buffer
						err := machine.Run(strings.NewReader(tc.Input), &out)
						message := ""
						if err != nil {
							message = err.Error()
						}
						if out.String() != tc.Expected.Output || message != tc.Expected.Message {
							t.Fatalf("out=%q err=%q want=%q / %q", out.String(), message, tc.Expected.Output, tc.Expected.Message)
						}
					}
				})
			}
		})
	}
	if shared != 22 || switches != 44 {
		t.Fatalf("shared=%d switch=%d", shared, switches)
	}
}
