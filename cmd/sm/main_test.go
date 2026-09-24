package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if stdout.String() != usage || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestInvalidArguments(t *testing.T) {
	cases := [][]string{
		nil, {"code"}, {"code", "input", "extra"},
		{"-code", "code"}, {"-input", "input"}, {"-code"},
		{"-unknown"}, {"-code", "code", "input"},
		{"-code", "code", "-input", "input", "extra"},
		{"code", "-input", "input"}, {"code", "-input"},
		{"", "input"}, {"-code", "", "-input", "input"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestMissingFiles(t *testing.T) {
	dir := t.TempDir()
	program := writeFile(t, dir, "code.json", `[]`)
	missing := filepath.Join(dir, "missing")
	for _, args := range [][]string{{missing, missing}, {program, missing}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
}

func TestDiagnosticPrintedOnce(t *testing.T) {
	dir := t.TempDir()
	program := writeFile(t, dir, "code.json", `["WRITE"]`)
	input := writeFile(t, dir, "input.txt", "")
	var stdout, stderr bytes.Buffer
	if code := run([]string{program, input}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code %d", code)
	}
	if stdout.Len() != 0 || strings.Count(stderr.String(), "stack underflow") != 1 || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestInputWhitespaceAndExtraValues(t *testing.T) {
	dir := t.TempDir()
	program := writeFile(t, dir, "code.json", `["READ","WRITE"]`)
	input := writeFile(t, dir, "input.txt", " \r\n -3, 0,\n 2147483647 \t")
	var stdout, stderr bytes.Buffer
	if code := run([]string{program, input}, &stdout, &stderr); code != 0 || stdout.String() != "-3" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func writeFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
