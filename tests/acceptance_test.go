package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testCase struct {
	Name       string          `json:"name"`
	Source     string          `json:"source,omitempty"`
	Program    json.RawMessage `json:"program,omitempty"`
	RawProgram *string         `json:"raw_program,omitempty"`
	Input      string          `json:"input"`
	Output     string          `json:"output"`
	Error      string          `json:"error,omitempty"`
}

func fixturePath(tc testCase, name string) string {
	return filepath.Join("testdata", "cases", tc.Name, name)
}

func readFixture(t *testing.T, tc testCase, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath(tc, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	var all []testCase
	for _, name := range []string{"compiled.json", "machine.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var cases []testCase
		if err := decoder.Decode(&cases); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			t.Fatalf("%s: extra data after cases", name)
		}
		if len(cases) == 0 {
			t.Fatalf("%s has no cases", name)
		}
		all = append(all, cases...)
	}
	return all
}

// This target is useful before an interpreter exists. It checks fixtures only.
func TestFixtures(t *testing.T) {
	names := map[string]bool{}
	validName := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	validErrors := map[string]bool{
		"": true, "undefined_variable": true, "input_exhausted": true,
		"division_by_zero": true, "stack_underflow": true,
		"undefined_label": true, "duplicate_label": true,
		"invalid_input": true, "invalid_program": true,
	}
	for _, tc := range loadCases(t) {
		if !validName.MatchString(tc.Name) || names[tc.Name] {
			t.Errorf("invalid or duplicate case name %q", tc.Name)
		}
		names[tc.Name] = true
		if (len(tc.Program) == 0) == (tc.RawProgram == nil) {
			t.Errorf("%s: supply exactly one of program/raw_program", tc.Name)
		}
		if !validErrors[tc.Error] {
			t.Errorf("%s: unknown error category %q", tc.Name, tc.Error)
		}
		code := readFixture(t, tc, "code.json")
		if tc.RawProgram != nil {
			if string(code) != *tc.RawProgram {
				t.Errorf("%s: code.json differs from raw_program", tc.Name)
			}
		} else {
			var actual, expected bytes.Buffer
			if err := json.Compact(&actual, code); err != nil {
				t.Errorf("%s: invalid fixture JSON: %v", tc.Name, err)
			}
			if err := json.Compact(&expected, tc.Program); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual.Bytes(), expected.Bytes()) {
				t.Errorf("%s: code.json differs from catalogue", tc.Name)
			}
		}
		if string(readFixture(t, tc, "input.txt")) != tc.Input {
			t.Errorf("%s: input.txt differs from catalogue", tc.Name)
		}
		if strings.TrimSpace(string(readFixture(t, tc, "output.txt"))) != tc.Output {
			t.Errorf("%s: output.txt differs from catalogue", tc.Name)
		}
		if tc.Error != "" && tc.Output != "" {
			t.Errorf("%s: output on failure is intentionally unspecified", tc.Name)
		}
		if tc.Error == "" {
			if tc.RawProgram != nil {
				t.Errorf("%s: raw_program is reserved for malformed JSON", tc.Name)
			}
			if err := validateProgram(tc.Program); err != nil {
				t.Errorf("%s: %v", tc.Name, err)
			}
		}
		if tc.Output != "" {
			for _, value := range strings.Split(tc.Output, ";") {
				n, err := strconv.ParseInt(value, 10, 32)
				if err != nil || strconv.FormatInt(n, 10) != value {
					t.Errorf("%s: invalid expected output value %q", tc.Name, value)
				}
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join("testdata", "cases"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !names[entry.Name()] {
			t.Errorf("unexpected fixture entry: %s", entry.Name())
		}
	}
	t.Logf("validated %d cases", len(names))
}

func validateProgram(data []byte) error {
	var instructions []json.RawMessage
	if err := json.Unmarshal(data, &instructions); err != nil || instructions == nil {
		return fmt.Errorf("successful case must contain an instruction array")
	}
	for index, instruction := range instructions {
		var opcode string
		if json.Unmarshal(instruction, &opcode) == nil && (opcode == "READ" || opcode == "WRITE") {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(instruction, &object) != nil || len(object) != 1 {
			return fmt.Errorf("instruction %d: expected a single-key object or READ/WRITE", index)
		}
		for op, arg := range object {
			if op == "CONST" {
				var n int32
				if string(arg) == "null" || json.Unmarshal(arg, &n) != nil {
					return fmt.Errorf("instruction %d: invalid CONST", index)
				}
				continue
			}
			var value string
			if json.Unmarshal(arg, &value) != nil || value == "" {
				return fmt.Errorf("instruction %d: invalid operand", index)
			}
			switch op {
			case "LD", "ST", "LABEL", "JMP", "JZ", "JNZ":
			case "BINOP":
				if !strings.Contains("|+|-|*|/|%|==|!=|<|<=|>|>=|&&|!!|", "|"+value+"|") {
					return fmt.Errorf("instruction %d: invalid BINOP", index)
				}
			default:
				return fmt.Errorf("instruction %d: unknown opcode %q", index, op)
			}
		}
	}
	return nil
}

func interpreter(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("SM_BIN"); configured != "" {
		// Relative paths are resolved from the repository root, not ./tests.
		if !filepath.IsAbs(configured) {
			configured = filepath.Join("..", configured)
		}
		absolute, err := filepath.Abs(configured)
		if err != nil {
			t.Fatal(err)
		}
		return absolute
	}
	if _, err := os.Stat(filepath.Join("..", "cmd", "sm")); err != nil {
		t.Fatal("Go interpreter is not implemented yet: add cmd/sm (CLI: -code FILE -input FILE, or two positional paths), or set SM_BIN to an executable. Use make test-cases / make test-reference to check fixtures independently.")
	}
	binary := filepath.Join(t.TempDir(), "sm")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/sm")
	command.Dir = ".."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build interpreter: %v\n%s", err, output)
	}
	return binary
}

func TestInterpreter(t *testing.T) {
	cases := loadCases(t)
	binary := interpreter(t)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			for _, style := range []string{"flags", "positional"} {
				t.Run(style, func(t *testing.T) { runCase(t, binary, tc, style) })
			}
		})
	}
}

func runCase(t *testing.T, binary string, tc testCase, style string) {
	t.Helper()
	// Include spaces to catch implementations that split or mishandle paths.
	directory := filepath.Join(t.TempDir(), "files with spaces")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"code.json", "input.txt"} {
		if err := os.WriteFile(filepath.Join(directory, filename), readFixture(t, tc, filename), 0600); err != nil {
			t.Fatal(err)
		}
	}
	codePath, inputPath := filepath.Join(directory, "code.json"), filepath.Join(directory, "input.txt")
	args := []string{"-code", codePath, "-input", inputPath}
	if style == "positional" {
		args = []string{codePath, inputPath}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("interpreter exceeded 3s (possible infinite loop); stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if tc.Error != "" {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() <= 0 || strings.TrimSpace(stderr.String()) == "" {
			t.Fatalf("want nonzero exit and diagnostic for %s; err=%v stdout=%q stderr=%q", tc.Error, err, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "panic:") || strings.Contains(stderr.String(), "fatal error:") {
			t.Fatalf("unhandled Go failure instead of a diagnostic: %s", stderr.String())
		}
		return
	}
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("want success; err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	parts := strings.Split(strings.TrimSpace(stdout.String()), ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	expected := strings.TrimSpace(string(readFixture(t, tc, "output.txt")))
	if actual := strings.Join(parts, ";"); actual != expected {
		t.Errorf("output: got %q, want %q", stdout.String(), expected)
	}
}
