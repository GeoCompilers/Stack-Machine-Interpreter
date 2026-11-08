package runtime

import (
	"encoding/json"
	"testing"
)

func decodedPattern(t *testing.T, text string) *Pattern {
	t.Helper()
	p, err := DecodePattern(json.RawMessage(text))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPatternMatching(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		value   Value
		want    bool
	}{
		{`1`, int32(1), true}, {`-1`, int32(-1), true}, {`1`, NewString("1"), false},
		{`""`, NewString(""), true}, {`"я"`, NewString("я"), true}, {`"x"`, NewString("y"), false},
		{`"#int"`, int32(1), true}, {`"#int"`, NewString("#int"), false},
		{`"#string"`, NewString("#int"), true}, {`"#string"`, int32(1), false},
		{`"#array"`, NewArray(nil), true}, {`"#array"`, NewSExpression("nil", nil), false},
		{`"#sexp"`, NewSExpression("nil", nil), true}, {`"#sexp"`, NewArray(nil), false},
		{`"#other"`, NewString("#other"), true},
		{`{"array":[]}`, NewArray(nil), true}, {`{"array":[]}`, NewArray([]Value{int32(1)}), false},
		{`{"array":[1,"x"]}`, NewArray([]Value{int32(1), NewString("x")}), true},
		{`{"sexp":"Leaf","args":[]}`, NewSExpression("Leaf", nil), true},
		{`{"sexp":"Leaf","args":[]}`, NewSExpression("Other", nil), false},
		{`{"sexp":"Leaf","args":[]}`, NewSExpression("Leaf", []Value{int32(1)}), false},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			bindings, ok := Match(decodedPattern(t, tc.pattern), tc.value)
			if ok != tc.want {
				t.Fatalf("matched=%v, want %v", ok, tc.want)
			}
			if !ok && bindings != nil {
				t.Fatal("failed match leaked bindings")
			}
		})
	}
}

func TestPatternBindings(t *testing.T) {
	duplicate := decodedPattern(t, `{"array":[{"bind":{"pattern":{"wildcard":[]},"name":"x"}},{"bind":{"pattern":{"wildcard":[]},"name":"x"}}]}`)
	for _, n := range []int32{2, 3} {
		bindings, ok := Match(duplicate, NewArray([]Value{int32(1), n}))
		if !ok || bindings["x"] != n {
			t.Fatalf("bindings=%v matched=%v", bindings, ok)
		}
	}
	outer := decodedPattern(t, `{"bind":{"name":"x","pattern":{"sexp":"Box","args":[{"bind":{"name":"x","pattern":{"wildcard":[]}}}]}}}`)
	object := NewSExpression("Box", []Value{int32(1)})
	bindings, ok := Match(outer, object)
	if !ok || bindings["x"] != object {
		t.Fatal("outer alias must override inner binding")
	}
	partial := decodedPattern(t, `{"array":[{"bind":{"name":"x","pattern":{"wildcard":[]}}},2]}`)
	bindings, ok = Match(partial, NewArray([]Value{int32(1), int32(3)}))
	if ok || bindings != nil {
		t.Fatal("failed match leaked a partial binding")
	}
}

func TestPatternsOnCycles(t *testing.T) {
	a := NewArray([]Value{int32(0)})
	if err := Set(a, int32(0), a); err != nil {
		t.Fatal(err)
	}
	p := decodedPattern(t, `{"array":[{"bind":{"pattern":{"wildcard":[]},"name":"x"}}]}`)
	bindings, ok := Match(p, a)
	if !ok || bindings["x"] != a {
		t.Fatal("finite pattern must match a cyclic graph without traversal")
	}
	if err := MatchFailure(a); err != errCycle {
		t.Fatalf("cyclic failure formatting: %v", err)
	}
	b := NewSExpression("Box", []Value{int32(0)})
	if err := Set(b, int32(0), b); err != nil {
		t.Fatal(err)
	}
	if _, ok := Match(decodedPattern(t, `"#sexp"`), b); !ok {
		t.Fatal("type matching must not traverse the graph")
	}
	if err := MatchFailure(b); err != errSExpressionCycle {
		t.Fatalf("cyclic failure formatting: %v", err)
	}
}

func TestInvalidPatterns(t *testing.T) {
	for _, data := range []string{
		"", `null`, `true`, `false`, `[]`, `1.5`, `1e0`, `2147483648`, `-2147483649`, `{}`, `"x" "y"`,
		`{"const":1}`, `{"string":"#int"}`, `{"wildcard":null}`, `{"wildcard":{}}`, `{"wildcard":[1]}`,
		`{"wildcard":[],"extra":1}`, `{"array":null}`, `{"array":1}`, `{"array":[null]}`,
		`{"sexp":"Leaf"}`, `{"sexp":null,"args":[]}`, `{"sexp":"","args":[]}`,
		`{"sexp":"Box","args":null}`, `{"sexp":"Box","args":[true]}`, `{"sexp":"Box","args":[],"extra":0}`,
		`{"bind":null}`, `{"bind":{"name":"x"}}`, `{"bind":{"name":"","pattern":{"wildcard":[]}}}`,
		`{"bind":{"name":null,"pattern":{"wildcard":[]}}}`, `{"bind":{"name":"x","pattern":null}}`,
		`{"bind":{"name":"x","pattern":{"wildcard":[]},"extra":1}}`,
	} {
		t.Run(data, func(t *testing.T) {
			if _, err := DecodePattern(json.RawMessage(data)); err == nil {
				t.Fatal("accepted invalid pattern")
			}
		})
	}
}
