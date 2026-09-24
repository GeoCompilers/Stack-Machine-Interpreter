package stackmachine

import (
	"bytes"
	"errors"
	"io"
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
		var stdout, stderr bytes.Buffer
		if err := machine.Run([]int32{value}, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		expected := "7"
		if value == -3 {
			expected = "-3"
		}
		if stdout.String() != expected || stderr.Len() != 0 {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
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
		{"stack", `["READ",{"JZ":"reuse"},{"CONST":9},{"JMP":"end"},{"LABEL":"reuse"},"WRITE",{"LABEL":"end"}]`, false, "stack underflow"},
		{"memory", `["READ",{"JZ":"reuse"},{"CONST":9},{"ST":"x"},{"JMP":"end"},{"LABEL":"reuse"},{"LD":"x"},"WRITE",{"LABEL":"end"}]`, false, "unknown variable"},
		{"after_failure", `["READ",{"JZ":"reuse"},{"CONST":9},{"ST":"x"},"READ",{"LABEL":"reuse"},{"LD":"x"},"WRITE"]`, true, "unknown variable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := newMachine(t, tc.program)
			if err := machine.Run([]int32{1}, io.Discard, io.Discard); (err != nil) != tc.firstError {
				t.Fatalf("first run: %v", err)
			}
			var stdout, stderr bytes.Buffer
			err := machine.Run([]int32{0}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.secondError) {
				t.Fatalf("second run: %v", err)
			}
			if stdout.Len() != 0 || stderr.String() != err.Error()+"\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
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
			var stderr bytes.Buffer
			err := machine.Run(nil, tc.writer, &stderr)
			if !errors.Is(err, tc.want) || stderr.String() != err.Error()+"\n" {
				t.Fatalf("err=%v stderr=%q", err, stderr.String())
			}
		})
	}
}

func TestDiagnosticWriterFailure(t *testing.T) {
	failure := errors.New("diagnostic unavailable")
	machine := newMachine(t, `["WRITE"]`)
	err := machine.Run(nil, io.Discard, failingWriter{failure})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "stack underflow") {
		t.Fatalf("expected execution and diagnostic errors, got %v", err)
	}
}

func newMachine(t *testing.T, program string) *StackMachine {
	t.Helper()
	machine, err := NewStackMachine([]byte(program))
	if err != nil {
		t.Fatal(err)
	}
	return machine
}
