package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func call(args ...string) (int, string, string) {
	var out, err bytes.Buffer
	status := run(args, &out, &err)
	return status, out.String(), err.String()
}

func TestCLIOutput(t *testing.T) {
	code := fixture(t, "program.json", `{"seq":{"left":{"read":"x"},"right":{"write":{"var":"x"}}}}`)
	input := fixture(t, "input.txt", `[1,"hello",[]]`)
	status, out, diagnostic := call("-code", code, "-input", input)
	if status != 0 || out != "[1; hello; []]" || diagnostic != "" {
		t.Fatalf("%d %q %q", status, out, diagnostic)
	}
	destination := filepath.Join(t.TempDir(), "result.txt")
	status, out, diagnostic = call("-code", code, "-input", input, "-output", destination)
	if status != 0 || out != "" || diagnostic != "" {
		t.Fatalf("%d %q %q", status, out, diagnostic)
	}
	b, err := os.ReadFile(destination)
	if err != nil || string(b) != "[1; hello; []]" {
		t.Fatalf("result=%q error=%v", b, err)
	}
}

func TestCLIPreservesOutputOnFailure(t *testing.T) {
	input := fixture(t, "input.txt", "")
	for _, program := range []string{
		`{"write":{"var":"missing"}}`,
		`{"seq":{"left":{"write":{"const":7}},"right":{"write":{"var":"missing"}}}}`,
		`{"bogus":1}`,
	} {
		t.Run(program, func(t *testing.T) {
			code := fixture(t, "program.json", program)
			destination := fixture(t, "result.txt", "keep existing contents")
			status, out, diagnostic := call("-code", code, "-input", input, "-output", destination)
			if status != 1 || out != "" || diagnostic == "" {
				t.Fatalf("%d %q %q", status, out, diagnostic)
			}
			b, err := os.ReadFile(destination)
			if err != nil || string(b) != "keep existing contents" {
				t.Fatalf("result=%q error=%v", b, err)
			}
			missing := filepath.Join(t.TempDir(), "missing.txt")
			call("-code", code, "-input", input, "-output", missing)
			if _, err = os.Stat(missing); !os.IsNotExist(err) {
				t.Fatalf("failed program created output: %v", err)
			}
		})
	}
	code := fixture(t, "program.json", `{"write":{"const":7}}`)
	badInput := fixture(t, "input.txt", "bad input")
	destination := fixture(t, "result.txt", "keep")
	status, _, _ := call("-code", code, "-input", badInput, "-output", destination)
	b, _ := os.ReadFile(destination)
	if status != 1 || string(b) != "keep" {
		t.Fatalf("invalid input: status=%d file=%q", status, b)
	}
}

func TestCLIArgumentsAndFileErrors(t *testing.T) {
	code := fixture(t, "program.json", `{"write":{"const":1}}`)
	input := fixture(t, "input.txt", "")
	for _, args := range [][]string{
		{}, {"-code", code}, {"-input", input}, {code, input},
		{"-code", code, "-input", input, "extra"},
		{"-tree", "-code", code, "-input", input},
		{"-code", "-", "-input", input}, {"-code", code, "-input", "-"},
		{"-code", code, "-input", input, "-output", ""}, {"-code", code, "-input", input, "-output", "-"},
		{"-code", filepath.Join(t.TempDir(), "missing"), "-input", input},
		{"-code", code, "-input", filepath.Join(t.TempDir(), "missing")},
		{"-code", code, "-input", input, "-output", t.TempDir()},
	} {
		status, out, diagnostic := call(args...)
		if status != 1 || out != "" || diagnostic == "" {
			t.Fatalf("args=%q: %d %q %q", args, status, out, diagnostic)
		}
	}
	for _, flag := range []string{"-h", "-help", "--help"} {
		status, out, diagnostic := call(flag)
		if status != 0 || !strings.Contains(out, "Usage:") || diagnostic != "" {
			t.Fatalf("help: %d %q %q", status, out, diagnostic)
		}
	}
}

func TestCLIWriterFailures(t *testing.T) {
	code := fixture(t, "program.json", `{"write":{"const":1}}`)
	input := fixture(t, "input.txt", "")
	var diagnostic bytes.Buffer
	if status := run([]string{"-code", code, "-input", input}, failingWriter{}, &diagnostic); status != 1 || !strings.Contains(diagnostic.String(), "write output") {
		t.Fatalf("%d %q", status, diagnostic.String())
	}
	diagnostic.Reset()
	if status := run([]string{"--help"}, failingWriter{}, &diagnostic); status != 1 {
		t.Fatalf("help status=%d", status)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrPermission }

func TestExecutable(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "interpreter")
	build := exec.Command("go", "build", "-o", binary, ".")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	code := fixture(t, "program.json", `{"write":{"string":"hello"}}`)
	input := fixture(t, "input.txt", "")
	cmd := exec.Command(binary, "-code", code, "-input", input)
	var out, diagnostic bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil || out.String() != "hello" || diagnostic.Len() != 0 {
		t.Fatalf("out=%q stderr=%q error=%v", out.String(), diagnostic.String(), err)
	}
	missing := fixture(t, "missing.json", `{"write":{"var":"x"}}`)
	cmd = exec.Command(binary, "-code", missing, "-input", input)
	out.Reset()
	diagnostic.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || out.Len() != 0 || diagnostic.String() != "ERROR: variable \"x\" is undefined\n" {
		t.Fatalf("out=%q stderr=%q error=%v", out.String(), diagnostic.String(), err)
	}
}
