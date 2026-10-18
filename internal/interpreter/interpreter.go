// Package interpreter executes the JSON AST produced by the L0 eval.html page.
package interpreter

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type machine struct {
	variables  map[string]value
	input      []value
	inputIndex int
	output     []value
}

func Run(programJSON []byte, input io.Reader, output io.Writer) error {
	program, err := decodeStatement(programJSON)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	values, err := parseInput(string(data))
	if err != nil {
		return err
	}
	m := machine{variables: make(map[string]value), input: values}
	if err = m.exec(program); err != nil {
		return errors.New(decodeText(err.Error()))
	}
	results := make([]string, len(m.output))
	for i, v := range m.output {
		results[i], err = formatted(v)
		if err != nil {
			return err
		}
	}
	text := decodeText(strings.Join(results, "; "))
	if text == "" {
		return nil
	}
	n, err := io.WriteString(output, text)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if n != len(text) {
		return fmt.Errorf("write output: %w", io.ErrShortWrite)
	}
	return nil
}
