package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"stack-machine-interpreter/internal/stackmachine"
)

const usage = `Usage:
  main.exe -code "path_to_stack_instructions" -input "path_to_input_file"
  main.exe "path_to_stack_instructions" "path_to_input_file"
  main.exe --help

Code: a JSON array of stack machine instructions.
Input: comma-separated int32 values, strings and nested arrays; an empty file is allowed.
Output: WRITE results separated by "; ", formatted after successful execution.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	codePath, inputPath, err := parseArgs(args)
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
		return report(stderr, fmt.Errorf("read code: %w", err))
	}
	machine, err := stackmachine.NewStackMachine(code)
	if err != nil {
		return report(stderr, err)
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return report(stderr, fmt.Errorf("read input: %w", err))
	}
	defer input.Close()
	if err := machine.Run(input, stdout); err != nil {
		return report(stderr, err)
	}
	return 0
}

func parseArgs(args []string) (string, string, error) {
	flags := flag.NewFlagSet("main.exe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	code := flags.String("code", "", "path to instructions")
	input := flags.String("input", "", "path to input")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	if flags.NFlag() == 0 && flags.NArg() == 2 {
		*code, *input = flags.Arg(0), flags.Arg(1)
		if strings.HasPrefix(*code, "-") || strings.HasPrefix(*input, "-") {
			return "", "", fmt.Errorf("do not mix flags and positional paths")
		}
	} else if flags.NArg() != 0 || flags.NFlag() != 2 {
		return "", "", fmt.Errorf("provide -code and -input, or exactly two paths; see --help")
	}
	if *code == "" || *input == "" {
		return "", "", fmt.Errorf("code and input paths must not be empty")
	}
	return *code, *input, nil
}

func report(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	return 1
}
