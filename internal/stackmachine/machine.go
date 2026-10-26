package stackmachine

import (
	"errors"
	"fmt"
	"io"
	"stack-machine-interpreter/internal/runtime"
)

var errStack = errors.New("ERROR: stack exhausted")

type StackMachine struct {
	stack              []runtime.Value
	memory             map[string]runtime.Value
	instructions       []instruction
	instructionPointer int32
	labels             map[string]int32
}

func (m *StackMachine) Run(input io.Reader, output io.Writer) error {
	m.stack = nil
	m.memory = make(map[string]runtime.Value)
	m.instructionPointer = 0
	values, err := runtime.ReadInput(input)
	if err != nil {
		return err
	}
	results, err := m.run(values)
	if err != nil {
		return runtime.NormalizeError(err)
	}
	return runtime.WriteOutput(output, results)
}

func (m *StackMachine) run(input []runtime.Value) ([]runtime.Value, error) {
	inputIndex := 0
	var output []runtime.Value
	for int(m.instructionPointer) < len(m.instructions) {
		inst := m.instructions[m.instructionPointer]
		var err error
		switch inst.opcode {
		case "READ":
			if inputIndex == len(input) {
				err = errors.New("ERROR: input exhausted")
			} else {
				m.stack = append(m.stack, input[inputIndex])
				inputIndex++
			}
		case "WRITE":
			var v runtime.Value
			v, err = m.pop()
			if err == nil {
				output = append(output, v)
			}
		case "CONST":
			m.stack = append(m.stack, inst.value)
		case "STRING":
			m.stack = append(m.stack, runtime.NewString(inst.operand))
		case "ARRAY":
			err = m.makeArray(inst.value)
		case "LENGTH", "ELEM", "SET":
			err = m.aggregate(inst.opcode)
		case "LD":
			err = m.load(inst.operand)
		case "ST":
			err = m.store(inst.operand)
		case "BINOP":
			err = m.binary(inst.operand)
		case "JMP", "JZ", "JNZ":
			err = m.jump(inst.opcode, inst.operand)
		case "LABEL":
		}
		if err != nil {
			return nil, err
		}
		m.instructionPointer++
	}
	return output, nil
}

func (m *StackMachine) pop() (runtime.Value, error) {
	if len(m.stack) == 0 {
		return nil, errStack
	}
	i := len(m.stack) - 1
	v := m.stack[i]
	m.stack[i] = nil
	m.stack = m.stack[:i]
	return v, nil
}

func (m *StackMachine) load(name string) error {
	v, exists := m.memory[name]
	if !exists {
		return fmt.Errorf("ERROR: variable \"%s\" is undefined", name)
	}
	m.stack = append(m.stack, v)
	return nil
}

func (m *StackMachine) store(name string) error {
	v, err := m.pop()
	if err != nil {
		return err
	}
	m.memory[name] = v
	return nil
}

func (m *StackMachine) replace(count int, value runtime.Value) {
	i := len(m.stack) - count
	clear(m.stack[i:])
	m.stack = append(m.stack[:i], value)
}

func (m *StackMachine) binary(operator string) error {
	if len(m.stack) < 2 {
		return errStack
	}
	i := len(m.stack) - 2
	v, err := runtime.Binary(operator, m.stack[i], m.stack[i+1])
	if err != nil {
		return err
	}
	m.replace(2, v)
	return nil
}

func (m *StackMachine) jump(opcode, label string) error {
	if opcode != "JMP" {
		v, err := m.pop()
		if err != nil {
			return err
		}
		condition, err := runtime.Integer(v)
		if err != nil {
			return err
		}
		if opcode == "JZ" && condition != 0 || opcode == "JNZ" && condition == 0 {
			return nil
		}
	}
	m.instructionPointer = m.labels[label]
	return nil
}

func (m *StackMachine) makeArray(count int32) error {
	if int64(count) > int64(len(m.stack)) {
		return errStack
	}
	value := runtime.NewArray(m.stack[len(m.stack)-int(count):])
	m.replace(int(count), value)
	return nil
}

func (m *StackMachine) aggregate(opcode string) error {
	count := 1
	if opcode == "ELEM" {
		count = 2
	} else if opcode == "SET" {
		count = 3
	}
	if len(m.stack) < count {
		return errStack
	}
	i := len(m.stack) - count
	var value runtime.Value
	var err error
	switch opcode {
	case "LENGTH":
		value, err = runtime.Length(m.stack[i])
	case "ELEM":
		value, err = runtime.Element(m.stack[i], m.stack[i+1])
	case "SET":
		err = runtime.Set(m.stack[i], m.stack[i+1], m.stack[i+2])
	}
	if err != nil {
		return err
	}
	if opcode == "SET" {
		clear(m.stack[i:])
		m.stack = m.stack[:i]
	} else {
		m.replace(count, value)
	}
	return nil
}
