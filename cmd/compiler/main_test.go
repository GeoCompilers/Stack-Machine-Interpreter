package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stack-machine-interpreter/internal/stackmachine"
)

func fixture(t *testing.T, name, data string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(filename, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func call(args ...string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	status := run(args, &out, &diagnostic)
	return status, out.String(), diagnostic.String()
}

func TestCLICompilesASTToExecutableInstructions(t *testing.T) {
	ast := fixture(t, "program.json", `{"seq":{"left":{"read":"x"},"right":{"write":{"binop":"-","left":{"var":"x"},"right":{"const":2}}}}}`)
	status, out, diagnostic := call("-code", ast)
	if status != 0 || diagnostic != "" {
		t.Fatalf("status=%d stderr=%q", status, diagnostic)
	}
	machine, err := stackmachine.NewStackMachine([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	var result, errors bytes.Buffer
	if err := machine.Run(strings.NewReader("42"), &result); err != nil || result.String() != "40" {
		t.Fatalf("output=%q err=%v stderr=%q", result.String(), err, errors.String())
	}
	target := filepath.Join(t.TempDir(), "code.json")
	status, fileOut, diagnostic := call("-code", ast, "-output", target)
	data, err := os.ReadFile(target)
	if status != 0 || fileOut != "" || diagnostic != "" || err != nil || string(data) != out {
		t.Fatalf("status=%d out=%q stderr=%q file=%q err=%v", status, fileOut, diagnostic, data, err)
	}
}

func TestCLIPreservesOutputOnInvalidAST(t *testing.T) {
	for _, ast := range []string{`{write(1);}`, `{"write":null}`, `{"write":{"string":null}}`} {
		code := fixture(t, "program.json", ast)
		target := fixture(t, "code.json", "preserved")
		status, out, diagnostic := call("-code", code, "-output", target)
		data, err := os.ReadFile(target)
		if status != 1 || out != "" || diagnostic == "" || err != nil || string(data) != "preserved" {
			t.Fatalf("AST=%s: status=%d out=%q stderr=%q file=%q err=%v", ast, status, out, diagnostic, data, err)
		}
	}
}

func TestCLIArgumentsAndIOErrors(t *testing.T) {
	ast := fixture(t, "program.json", `"skip"`)
	for _, args := range [][]string{
		{}, {ast}, {"-source", ast}, {"-output", "result"}, {"-code", ""},
		{"-code", ast, "extra"}, {"-code", filepath.Join(t.TempDir(), "missing")},
		{"-code", ast, "-output", t.TempDir()},
	} {
		status, out, diagnostic := call(args...)
		if status != 1 || out != "" || diagnostic == "" {
			t.Fatalf("args=%q: status=%d out=%q stderr=%q", args, status, out, diagnostic)
		}
	}
	status, out, diagnostic := call("--help")
	if status != 0 || !strings.Contains(out, "-code") || diagnostic != "" {
		t.Fatalf("help: status=%d out=%q stderr=%q", status, out, diagnostic)
	}
	var errors bytes.Buffer
	for _, args := range [][]string{{"-code", ast}, {"--help"}} {
		if status := run(args, failingWriter{}, &errors); status != 1 {
			t.Fatalf("writer failure status=%d", status)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrPermission }
