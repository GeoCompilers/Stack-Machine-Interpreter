package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"stack-machine-interpreter/internal/interpreter"
)

const usage = `Usage:
  interpreter.exe -code "program.json" -input "input.txt" [-output "result.txt"]
  interpreter.exe --help

Code is the JSON AST from the Simple Imperative Language Reference Interpreter.
Input is a comma-separated list of integers, strings and arrays.
Output defaults to stdout. No trailing newline is added.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("interpreter.exe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	code := flags.String("code", "", "AST file")
	input := flags.String("input", "", "input file")
	output := flags.String("output", "", "output file")
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, err = io.WriteString(stdout, usage); err != nil {
			return report(stderr, err)
		}
		return 0
	}
	if err != nil {
		return report(stderr, err)
	}
	if flags.NArg() != 0 || *code == "" || *input == "" || *code == "-" || *input == "-" {
		return report(stderr, fmt.Errorf("provide -code FILE and -input FILE with optional -output FILE; see --help"))
	}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "output" && (*output == "" || *output == "-") {
			err = fmt.Errorf("-output must name a file")
		}
	})
	if err != nil {
		return report(stderr, err)
	}
	program, err := os.ReadFile(*code)
	if err != nil {
		return report(stderr, fmt.Errorf("read code: %w", err))
	}
	in, err := os.Open(*input)
	if err != nil {
		return report(stderr, fmt.Errorf("read input: %w", err))
	}
	defer in.Close()
	var result bytes.Buffer
	destination := stdout
	if *output != "" {
		destination = &result
	}
	if err = interpreter.Run(program, in, destination); err != nil {
		return report(stderr, err)
	}
	if *output != "" {
		if err = os.WriteFile(*output, result.Bytes(), 0600); err != nil {
			return report(stderr, fmt.Errorf("write output: %w", err))
		}
	}
	return 0
}
func report(stderr io.Writer, err error) int { _, _ = fmt.Fprintln(stderr, err); return 1 }
