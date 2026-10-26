package interpreter

import (
	"io"
	"stack-machine-interpreter/internal/runtime"
)

type machine struct {
	variables  map[string]runtime.Value
	input      []runtime.Value
	inputIndex int
	output     []runtime.Value
}

func Run(programJSON []byte, input io.Reader, output io.Writer) error {
	program, err := decodeStatement(programJSON)
	if err != nil {
		return err
	}
	values, err := runtime.ReadInput(input)
	if err != nil {
		return err
	}
	m := machine{variables: make(map[string]runtime.Value), input: values}
	if err := m.exec(program); err != nil {
		return runtime.NormalizeError(err)
	}
	return runtime.WriteOutput(output, m.output)
}
