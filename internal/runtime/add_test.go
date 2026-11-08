package runtime

import (
	"bytes"
	"testing"
)

func TestAdditionValues(t *testing.T) {
	for _, tc := range []struct {
		left, right Value
		want        Value
	}{
		{int32(1), int32(2), int32(3)}, {int32(2147483647), int32(1), int32(-2147483648)},
		{NewString("a"), NewString("b"), []byte("ab")},
		{int32(-2147483648), NewString("x"), []byte("-2147483648x")},
		{NewString("x"), int32(-12), []byte("x-12")},
		{[]byte{0xe2}, []byte{0x82}, []byte{0xe2, 0x82}},
	} {
		value, err := Binary("+", tc.left, tc.right)
		if err != nil {
			t.Fatal(err)
		}
		if want, ok := tc.want.([]byte); ok {
			got, ok := value.([]byte)
			if !ok || !bytes.Equal(got, want) {
				t.Fatalf("got=%v want=%v", value, want)
			}
		} else if value != tc.want {
			t.Fatalf("got=%v want=%v", value, tc.want)
		}
	}
}

func TestConcatenationOwnsStorage(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		original := make([]byte, 3, 32)
		copy(original, "abc")
		var left, right Value = original, []byte{}
		if prefix {
			left, right = right, left
		}
		value, err := Binary("+", left, right)
		if err != nil {
			t.Fatal(err)
		}
		joined := value.([]byte)
		joined[0] = 'Z'
		if string(original) != "abc" {
			t.Fatal("concatenation reused operand storage")
		}
		original[1] = 'Y'
		if string(joined) != "Zbc" {
			t.Fatal("result aliases original")
		}
	}
}

func TestAdditionTypeErrors(t *testing.T) {
	for _, tc := range []struct {
		left, right Value
		message     string
	}{
		{NewArray([]Value{int32(1)}), NewString("x"), `ERROR: binary operator "+" is applied to [1] and x`},
		{NewString("x"), NewSExpression("Leaf", nil), `ERROR: binary operator "+" is applied to x and Leaf`},
	} {
		if _, err := Binary("+", tc.left, tc.right); err == nil || err.Error() != tc.message {
			t.Fatalf("error=%v want=%q", err, tc.message)
		}
	}
}
