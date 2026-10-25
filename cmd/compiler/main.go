package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"stack-machine-interpreter/internal/compiler"
)

const usage = `Usage:
  compiler.exe -code "path_to_ast_json" [-output "path_to_sm_json"]
  compiler.exe --help

Without -output, the compiled stack machine instructions are written to stdout.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	codePath, outputPath, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return report(stderr, err)
		}
		return 0
	}
	if err != nil {
		return report(stderr, err)
	}
	code, err := os.ReadFile(codePath)
	if err != nil {
		return report(stderr, fmt.Errorf("read AST: %w", err))
	}
	program, err := compiler.Compile(code)
	if err != nil {
		return report(stderr, fmt.Errorf("compile: %w", err))
	}
	if outputPath == "" {
		if _, err := stdout.Write(program); err != nil {
			return report(stderr, fmt.Errorf("write program: %w", err))
		}
		return 0
	}
	if err := os.WriteFile(outputPath, program, 0600); err != nil {
		return report(stderr, fmt.Errorf("write program: %w", err))
	}
	return 0
}

func parseArgs(args []string) (string, string, error) {
	flags := flag.NewFlagSet("compiler.exe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	code := flags.String("code", "", "path to JSON AST")
	output := flags.String("output", "", "path to generated JSON")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	if flags.NArg() != 0 {
		return "", "", fmt.Errorf("provide -code FILE with optional -output FILE; see --help")
	}
	if *code == "" {
		return "", "", fmt.Errorf("code path must not be empty")
	}
	if *output != "" && filepath.Clean(*output) == "." {
		return "", "", fmt.Errorf("output path must name a file")
	}
	return *code, *output, nil
}

func report(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
