package interpreter

import "testing"

func sexp(name string, args ...any) any {
	if args == nil {
		args = []any{}
	}
	return map[string]any{"sexp": name, "args": args}
}

func TestSExpressionGraphs(t *testing.T) {
	x := variable("x")
	tail := variable("tail")
	for _, tc := range []struct {
		name, want, message string
		program             any
	}{
		{"shared_children", "Pair (Box (1), Box (1))", "", sequence(
			assign("x", sexp("Box", number(1))), write(sexp("Pair", x, x)))},
		{"head_is_shared_tail", "{{2}, 2}", "", sequence(
			assign("tail", sexp("cons", number(2), sexp("nil"))), write(sexp("cons", tail, tail)))},
		{"improper_list", "cons (1, 2)", "", write(sexp("cons", number(1), number(2)))},
		{"improper_tail", "cons (1, Other)", "", write(sexp("cons", number(1), sexp("Other")))},
		{"cons_wrong_arity", "cons (1)", "", write(sexp("cons", number(1)))},
		{"nil_with_args", "nil (1)", "", write(sexp("nil", number(1)))},
		{"cycle", "", "ERROR: cyclic S-expression", sequence(
			assign("x", sexp("Box", number(1))), set(x, number(0), x), write(x))},
		{"cycle_in_list_head", "", "ERROR: cyclic S-expression", sequence(
			assign("x", sexp("cons", number(1), sexp("nil"))), set(x, number(0), x), write(x))},
		{"cycle_in_list_tail", "", "ERROR: cyclic S-expression", sequence(
			assign("x", sexp("cons", number(1), sexp("nil"))), set(x, number(1), x), write(x))},
		{"cycle_through_array", "", "ERROR: cyclic S-expression", sequence(
			assign("x", sexp("Box", number(1))), set(x, number(0), node("array", []any{x})), write(x))},
		{"cycle_comparison", "", "ERROR: cyclic S-expression", sequence(
			assign("x", sexp("Box", number(1))), set(x, number(0), x), write(operation("==", x, x)))},
		{"deferred_cycle_discards_output", "", "ERROR: cyclic S-expression", sequence(
			write(number(7)), assign("x", sexp("Box", number(1))), write(x), set(x, number(0), x))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runProgram(t, tc.program, "")
			message := ""
			if err != nil {
				message = err.Error()
			}
			if out != tc.want || message != tc.message {
				t.Fatalf("output=%q, error=%q; want %q, %q", out, message, tc.want, tc.message)
			}
		})
	}
}
