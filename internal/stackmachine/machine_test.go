package stackmachine

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestInvalidPrograms(t *testing.T) {
	programs := []string{
		`[{"LD":""}]`, `[{"ST":""}]`, `[{"LABEL":""}]`,
		`[{"JMP":""}]`, `[{"JZ":""}]`, `[{"JNZ":""}]`,
		`[{"CONST":null}]`, `[{"LD":null}]`, `[{"CONST":1,"CONST":2}]`,
		`[{"LABEL":"same"},{"LABEL":"same"}]`,
		`[{"CONST":1},{"JZ":"missing"}]`,
		`[{"JMP":"end"},"INVALID",{"LABEL":"end"}]`,
	}
	for _, program := range programs {
		t.Run(program, func(t *testing.T) {
			if _, err := NewStackMachine([]byte(program)); err == nil {
				t.Fatal("expected invalid program error")
			}
		})
	}
}

func TestRepeatedRun(t *testing.T) {
	machine := newMachine(t, `["READ",{"ST":"x"},{"LD":"x"},"WRITE"]`)
	for _, value := range []int32{7, -3} {
		var stdout bytes.Buffer
		if err := machine.Run(strings.NewReader(strconv.Itoa(int(value))), &stdout); err != nil {
			t.Fatal(err)
		}
		expected := "7"
		if value == -3 {
			expected = "-3"
		}
		if stdout.String() != expected {
			t.Fatalf("stdout=%q", stdout.String())
		}
	}
}

func TestRunClearsState(t *testing.T) {
	cases := []struct {
		name        string
		program     string
		firstError  bool
		secondError string
	}{
		{"stack", `["READ",{"JZ":"reuse"},{"CONST":9},{"JMP":"end"},{"LABEL":"reuse"},"WRITE",{"LABEL":"end"}]`, false, "ERROR: stack exhausted"},
		{"memory", `["READ",{"JZ":"reuse"},{"CONST":9},{"ST":"x"},{"JMP":"end"},{"LABEL":"reuse"},{"LD":"x"},"WRITE",{"LABEL":"end"}]`, false, "is undefined"},
		{"after_failure", `["READ",{"JZ":"reuse"},{"CONST":9},{"ST":"x"},"READ",{"LABEL":"reuse"},{"LD":"x"},"WRITE"]`, true, "is undefined"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := newMachine(t, tc.program)
			if err := machine.Run(strings.NewReader("1"), io.Discard); (err != nil) != tc.firstError {
				t.Fatalf("first run: %v", err)
			}
			var stdout bytes.Buffer
			err := machine.Run(strings.NewReader("0"), &stdout)
			if err == nil || !strings.Contains(err.Error(), tc.secondError) {
				t.Fatalf("second run: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q", stdout.String())
			}
		})
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestOutputWriterFailure(t *testing.T) {
	failure := errors.New("output unavailable")
	for _, tc := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{"failure", failingWriter{failure}, failure},
		{"short_write", failingWriter{}, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := newMachine(t, `[{"CONST":1},"WRITE"]`)
			err := machine.Run(strings.NewReader(""), tc.writer)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestInputReaderFailure(t *testing.T) {
	failure := errors.New("input unavailable")
	machine := newMachine(t, `[]`)
	err := machine.Run(failingReader{failure}, io.Discard)
	if !errors.Is(err, failure) {
		t.Fatalf("expected input error, got %v", err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func newMachine(t *testing.T, program string) *StackMachine {
	t.Helper()
	machine, err := NewStackMachine([]byte(program))
	if err != nil {
		t.Fatal(err)
	}
	return machine
}
