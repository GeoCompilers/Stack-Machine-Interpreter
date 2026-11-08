package interpreter

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/runtime"
)

func TestLanguageReference(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/language_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name     string
			Program  json.RawMessage
			Input    string
			Expected struct{ Output, Message string }
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 66 {
		t.Fatalf("expected 66 cases, got %d", len(corpus.Cases))
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			for i := 0; i < 2; i++ {
				var out bytes.Buffer
				err := Run(tc.Program, strings.NewReader(tc.Input), &out)
				message := ""
				if err != nil {
					message = err.Error()
				}
				if message != tc.Expected.Message || out.String() != tc.Expected.Output {
					t.Fatalf("out=%q err=%q, want %q / %q", out.String(), message, tc.Expected.Output, tc.Expected.Message)
				}
			}
		})
	}
}

func TestInvalidSwitchAST(t *testing.T) {
	for _, payload := range []string{
		`null`, `{}`, `{"scrutinee":{"const":1}}`, `{"branches":[]}`,
		`{"scrutinee":null,"branches":[{"pattern":{"wildcard":[]},"branch":"skip"}]}`,
		`{"scrutinee":{"const":1},"branches":[]}`, `{"scrutinee":{"const":1},"branches":null}`,
		`{"scrutinee":{"const":1},"branches":{}}`, `{"scrutinee":{"const":1},"branches":[null]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":1}]}`,
		`{"scrutinee":{"const":1},"branches":[{"branch":"skip"}]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":null,"branch":"skip"}]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":{"wildcard":[]},"branch":null}]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":{"wildcard":[]},"branch":"skip","extra":0}]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":{"wildcard":[]},"branch":"skip"}],"extra":0}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":{"wildcard":[]},"branch":"skip"},{"pattern":null,"branch":"skip"}]}`,
		`{"scrutinee":{"const":1},"branches":[{"pattern":{"wildcard":[]},"branch":"skip"},{"pattern":1,"branch":{"unknown":0}}]}`,
	} {
		t.Run(payload, func(t *testing.T) {
			data := []byte(`{"seq":{"left":{"write":{"const":7}},"right":{"switch":` + payload + `}}}`)
			var out bytes.Buffer
			err := Run(data, strings.NewReader(""), &out)
			if err == nil || !strings.HasPrefix(err.Error(), "AST:") || out.Len() != 0 {
				t.Fatalf("out=%q err=%v", out.String(), err)
			}
		})
	}
}

func TestSwitchRestoresBindingsOnError(t *testing.T) {
	body := sequence(node("read", "x"), assign("outside", number(5)), assign("created", number(7)), write(variable("temporary")), write(variable("missing")))
	ast := node("switch", map[string]any{
		"scrutinee": sexp("Pair", number(1), number(2)),
		"branches": []any{map[string]any{
			"pattern": map[string]any{"sexp": "Pair", "args": []any{
				map[string]any{"bind": map[string]any{"name": "x", "pattern": map[string]any{"wildcard": []any{}}}},
				map[string]any{"bind": map[string]any{"name": "temporary", "pattern": map[string]any{"wildcard": []any{}}}},
			}}, "branch": body,
		}},
	})
	data, err := json.Marshal(ast)
	if err != nil {
		t.Fatal(err)
	}
	program, err := decodeStatement(data)
	if err != nil {
		t.Fatal(err)
	}
	m := machine{variables: map[string]runtime.Value{"x": int32(9), "outside": int32(3)}, input: []runtime.Value{int32(8)}}
	if err := m.exec(program); err == nil || err.Error() != `ERROR: variable "missing" is undefined` {
		t.Fatalf("error=%v", err)
	}
	if m.variables["x"] != int32(9) || m.variables["outside"] != int32(5) || m.variables["created"] != int32(7) {
		t.Fatalf("variables=%v", m.variables)
	}
	if _, ok := m.variables["temporary"]; ok {
		t.Fatal("local binding leaked on failure")
	}
	if m.inputIndex != 1 {
		t.Fatal("input consumption was rolled back")
	}
}

func TestSwitchMatchesCyclicValues(t *testing.T) {
	a := variable("a")
	pattern := map[string]any{"bind": map[string]any{"name": "x", "pattern": map[string]any{"wildcard": []any{}}}}
	for _, tc := range []struct {
		name            string
		body            any
		output, message string
	}{
		{"bind_without_traversal", write(node("length", variable("x"))), "1", ""},
		{"cyclic_output", write(variable("x")), "", "ERROR: cyclic array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program := sequence(assign("a", node("array", []any{number(0)})), set(a, number(0), a), node("switch", map[string]any{
				"scrutinee": a, "branches": []any{map[string]any{"pattern": pattern, "branch": tc.body}},
			}))
			data, err := json.Marshal(program)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = Run(data, strings.NewReader(""), &out)
			message := ""
			if err != nil {
				message = err.Error()
			}
			if out.String() != tc.output || message != tc.message {
				t.Fatalf("out=%q err=%q", out.String(), message)
			}
		})
	}
}
