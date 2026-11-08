package runtime

import (
	"bytes"
	"strings"
	"testing"
)

func TestStructuredInput(t *testing.T) {
	for _, tc := range []struct{ input, output string }{
		{`Pair(1,"hello",{2,3})`, `Pair (1, hello, {2, 3})`},
		{`Leaf, A_B12, Cons(1,Nil)`, `Leaf; A_B12; Cons (1, Nil)`},
		{`{}, [], {1,Pair(-2,3),[4]}`, `{}; []; {1, Pair (-2, 3), [4]}`},
		{`Box("a, b""c",{[],{}})`, `Box (a, b"c, {[], {}})`},
		{"'A', '\\n', '\\t', ''''", `65; 10; 9; 39`},
		{"' ', '\\'", `32; 92`},
		{"(* c (* nested *) *) Pair -- comment\n (1, {2, 3})", `Pair (1, {2, 3})`},
		{"\"a\\nb\"", `a\nb`},
		{"-- only a comment", ""},
	} {
		t.Run(tc.input, func(t *testing.T) {
			values, err := ReadInput(strings.NewReader(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := WriteOutput(&out, values); err != nil || out.String() != tc.output {
				t.Fatalf("out=%q err=%v want=%q", out.String(), err, tc.output)
			}
		})
	}
}

func TestInvalidStructuredInput(t *testing.T) {
	for _, input := range []string{
		`Leaf()`, `Box()`, `Pair(1,)`, `Pair(,1)`, `Pair(1`, `Pair(1]`, `Pair(1))`, `Leaf x`, `leaf`,
		`{1,}`, `{,1}`, `{1`,
		`1:2:{}`, `1+2`, `(1)`, `cons(1,nil)`, `[Leaf,]`, `{Box(1),}`, `Pair(2147483648,2)`,
		`''`, `'ab'`, `'я'`, `'🙂'`, `'a`, `'\r'`, `'\x41'`, `'''`, `'a'b`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := ReadInput(strings.NewReader(input)); err == nil || !strings.HasPrefix(err.Error(), "Input error:") {
				t.Fatalf("accepted %q: %v", input, err)
			}
		})
	}
}
