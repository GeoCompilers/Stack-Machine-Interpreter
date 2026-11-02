package runtime

import (
	"bytes"
	"strings"
	"testing"
)

func TestLongListOutput(t *testing.T) {
	list := NewSExpression("nil", nil)
	const count = 10000
	for i := 0; i < count; i++ {
		list = NewSExpression("cons", []Value{int32(1), list})
	}
	var out bytes.Buffer
	if err := WriteOutput(&out, []Value{list}); err != nil {
		t.Fatal(err)
	}
	want := "{" + strings.Repeat("1, ", count-1) + "1}"
	if out.String() != want {
		t.Fatal("long list was not formatted correctly")
	}
}
