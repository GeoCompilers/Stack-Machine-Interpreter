package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"stack-machine-interpreter/internal/compiler"
)

const usage = `Usage:
  compiler.exe -source "path_to_source" [-output "path_to_code_json"]
  compiler.exe "path_to_source" ["path_to_code_json"]
  compiler.exe --help

Without -output (or a second path), the compiled JSON program is written to stdout.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	sourcePath, outputPath, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, usage)
		return 0
	}
	if err != nil {
		return report(stderr, err)
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return report(stderr, fmt.Errorf("read source: %w", err))
	}
	program, err := compiler.Compile(string(source))
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
	source := flags.String("source", "", "path to source")
	output := flags.String("output", "", "path to generated JSON")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	if flags.NFlag() == 0 && (flags.NArg() == 1 || flags.NArg() == 2) {
		*source = flags.Arg(0)
		if flags.NArg() == 2 {
			*output = flags.Arg(1)
		}
		if strings.HasPrefix(*source, "-") || strings.HasPrefix(*output, "-") {
			return "", "", fmt.Errorf("do not mix flags and positional paths")
		}
	} else if flags.NArg() != 0 || flags.NFlag() < 1 || flags.NFlag() > 2 {
		return "", "", fmt.Errorf("provide -source FILE with optional -output FILE, or one/two paths; see --help")
	}
	if *source == "" {
		return "", "", fmt.Errorf("source path must not be empty")
	}
	if *output != "" && filepath.Clean(*output) == "." {
		return "", "", fmt.Errorf("output path must name a file")
	}
	return *source, *output, nil
}

func report(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
