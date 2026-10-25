package stackmachine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAggregateInstructions(t *testing.T) {
	for _, test := range []struct{ name, code, input, output, message string }{
		{"array_order", `[{"CONST":1},{"CONST":2},{"CONST":3},{"ARRAY":3},"WRITE"]`, "", "[1; 2; 3]", ""},
		{"array_preserves_lower_stack", `[{"CONST":99},{"CONST":1},{"CONST":2},{"ARRAY":2},"WRITE","WRITE"]`, "", "[1; 2]; 99", ""},
		{"empty_array", `[{"ARRAY":0},"WRITE"]`, "", "[]", ""},
		{"empty_array_preserves_stack", `[{"CONST":7},{"ARRAY":0},"WRITE","WRITE"]`, "", "[]; 7", ""},
		{"stack_backing_storage", `[{"CONST":1},{"CONST":2},{"ARRAY":2},{"ST":"a"},{"CONST":99},{"CONST":100},{"ARRAY":2},"WRITE",{"LD":"a"},"WRITE"]`, "", "[99; 100]; [1; 2]", ""},
		{"nested_mixed", `[{"CONST":1},{"STRING":"x"},{"CONST":2},{"ARRAY":1},{"ARRAY":3},"WRITE"]`, "", "[1; x; [2]]", ""},
		{"string_empty", `[{"STRING":""},"WRITE",{"CONST":1},"WRITE"]`, "", "; 1", ""},
		{"string_literal_quotes", `[{"STRING":"\"abc\""},"WRITE"]`, "", `"abc"`, ""},
		{"string_byte_length", `[{"STRING":"я🙂"},"LENGTH","WRITE"]`, "", "6", ""},
		{"string_byte_element", `[{"STRING":"я"},{"CONST":1},"ELEM","WRITE"]`, "", "143", ""},
		{"array_element", `[{"STRING":"x"},{"CONST":2},{"ARRAY":2},{"CONST":0},"ELEM","WRITE"]`, "", "x", ""},
		{"set_alias_and_deferred_write", `[{"STRING":"abc"},{"ST":"a"},{"LD":"a"},{"ST":"b"},{"LD":"a"},"WRITE",{"LD":"b"},{"CONST":1},{"CONST":90},"SET"]`, "", "aZc", ""},
		{"independent_string_literals", `[{"STRING":"abc"},{"ST":"a"},{"LD":"a"},"WRITE",{"LD":"a"},{"CONST":0},{"CONST":90},"SET",{"STRING":"abc"},"WRITE"]`, "", "Zbc; abc", ""},
		{"array_set_changes_type", `[{"CONST":1},{"ARRAY":1},{"ST":"a"},{"LD":"a"},"WRITE",{"LD":"a"},{"CONST":0},{"STRING":"x"},"SET"]`, "", "[x]", ""},
		{"set_leaves_no_result", `[{"CONST":8},{"STRING":"a"},{"CONST":0},{"CONST":90},"SET","WRITE"]`, "", "8", ""},
		{"read_mixed", `["READ","WRITE","READ","WRITE"]`, `"hello",[1,"x",[]]`, "hello; [1; x; []]", ""},
		{"read_quote", `["READ","WRITE"]`, `"a""b"`, `a"b`, ""},
		{"read_comments", `["READ","WRITE"]`, `(* nested (* comment *) *) [1,-- x
"a",[]]`, "[1; a; []]", ""},
		{"bad_input_before_execution", `[{"CONST":7},"WRITE"]`, "1,", "", "Input error:"},
		{"bounds", `[{"STRING":"ab"},{"CONST":2},"ELEM"]`, "", "", "ERROR: index 2 is out of bounds [0, 2)"},
		{"negative_index", `[{"ARRAY":0},{"CONST":-1},"ELEM"]`, "", "", "ERROR: index -1 is out of bounds [0, 0)"},
		{"index_type_first", `[{"CONST":1},{"STRING":"x"},"ELEM"]`, "", "", `ERROR: integer value expected but "x" given`},
		{"object_type", `[{"CONST":1},{"CONST":0},"ELEM"]`, "", "", `ERROR: array/string value expected but "1" is given`},
		{"length_type", `[{"CONST":1},"LENGTH"]`, "", "", `ERROR: array/string value expected but "1" is given`},
		{"set_bounds_before_value_type", `[{"STRING":"a"},{"CONST":1},{"ARRAY":0},"SET"]`, "", "", "ERROR: index 1 is out of bounds [0, 1)"},
		{"char_overflow", `[{"STRING":"a"},{"CONST":0},{"CONST":256},"SET"]`, "", "", `Invalid_argument("Char.chr")`},
		{"char_negative", `[{"STRING":"a"},{"CONST":0},{"CONST":-1},"SET"]`, "", "", `Invalid_argument("Char.chr")`},
		{"char_zero", `[{"STRING":"a"},{"ST":"s"},{"LD":"s"},{"CONST":0},{"CONST":0},"SET",{"LD":"s"},"WRITE"]`, "", "\x00", ""},
		{"char_255", `[{"STRING":"a"},{"ST":"s"},{"LD":"s"},{"CONST":0},{"CONST":255},"SET",{"LD":"s"},"WRITE"]`, "", "�", ""},
		{"truncated_utf8", `[{"STRING":"ab"},{"ST":"s"},{"LD":"s"},{"CONST":0},{"CONST":226},"SET",{"LD":"s"},{"CONST":1},{"CONST":130},"SET",{"LD":"s"},"WRITE"]`, "", "�", ""},
		{"condition_type", `[{"STRING":"a"},{"JZ":"end"},{"LABEL":"end"}]`, "", "", `ERROR: integer value expected but "a" given`},
		{"right_operand_type_first", `[{"STRING":"left"},{"STRING":"right"},{"BINOP":"+"}]`, "", "", `ERROR: integer value expected but "right" given`},
		{"partial_output_discarded", `[{"CONST":7},"WRITE",{"LD":"missing"},"WRITE"]`, "", "", `ERROR: variable "missing" is undefined`},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := newMachine(t, test.code)
			for i := 0; i < 2; i++ {
				var out bytes.Buffer
				err := machine.Run(strings.NewReader(test.input), &out)
				if test.message == "" && err != nil || test.message != "" && (err == nil || !strings.HasPrefix(err.Error(), test.message)) {
					t.Fatalf("err=%v, want %q", err, test.message)
				}
				if out.String() != test.output {
					t.Fatalf("output=%q, want %q", out.String(), test.output)
				}
			}
		})
	}
}

func TestAggregateUnderflow(t *testing.T) {
	for opcode, count := range map[string]int{"LENGTH": 1, "ELEM": 2, "SET": 3, "ARRAY": 4} {
		for available := 0; available < count; available++ {
			t.Run(fmt.Sprintf("%s_%d", opcode, available), func(t *testing.T) {
				instructions := make([]any, 0, available+1)
				for i := 0; i < available; i++ {
					instructions = append(instructions, map[string]int32{"CONST": int32(i)})
				}
				if opcode == "ARRAY" {
					instructions = append(instructions, map[string]int32{"ARRAY": int32(count)})
				} else {
					instructions = append(instructions, opcode)
				}
				code, _ := json.Marshal(instructions)
				machine := newMachine(t, string(code))
				var out bytes.Buffer
				err := machine.Run(strings.NewReader(""), &out)
				if err != errStack || out.Len() != 0 {
					t.Fatalf("err=%v, output=%q", err, out.String())
				}
			})
		}
	}
	machine := newMachine(t, `[{"ARRAY":2147483647}]`)
	if err := machine.Run(strings.NewReader(""), &bytes.Buffer{}); err != errStack {
		t.Fatalf("large count: %v", err)
	}
}

func TestInvalidAggregateInstructions(t *testing.T) {
	for _, code := range []string{
		`[{"ARRAY":-1}]`, `[{"ARRAY":null}]`, `[{"ARRAY":1.5}]`, `[{"ARRAY":2147483648}]`, `[{"ARRAY":"1"}]`,
		`[{"ARRAY":1,"ARRAY":2}]`, `["ARRAY"]`, `["STRING"]`, `[{"STRING":null}]`, `[{"STRING":1}]`,
		`[{"STRING":[]}]`, `[{"STRING":"x","STRING":"y"}]`, `[{"LENGTH":0}]`, `[{"ELEM":null}]`, `[{"SET":[]}]`,
	} {
		if _, err := NewStackMachine([]byte(code)); err == nil {
			t.Fatalf("accepted invalid instruction: %s", code)
		}
	}
}
