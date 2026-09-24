package stackmachine

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

type StackMachine struct {
	stack             []int32
	memory            map[string]int32
	instructions      []instruction
	isntrucionPointer int32
	labels            map[string]int32
}

func (m *StackMachine) Run(input []int32, stdout, stderr io.Writer) error {
	m.stack = nil
	m.memory = make(map[string]int32)
	m.isntrucionPointer = 0
	if err := m.run(input, stdout); err != nil {
		if writeErr := writeText(stderr, err.Error()+"\n"); writeErr != nil {
			return errors.Join(err, fmt.Errorf("write diagnostic: %w", writeErr))
		}
		return err
	}
	return nil
}

func (m *StackMachine) run(input []int32, stdout io.Writer) error {
	inputIndex := 0
	separator := ""
	for int(m.isntrucionPointer) < len(m.instructions) {
		inst := m.instructions[m.isntrucionPointer]
		var err error
		switch inst.opcode {
		case "READ":
			if inputIndex == len(input) {
				err = fmt.Errorf("input exhausted")
			} else {
				m.stack = append(m.stack, input[inputIndex])
				inputIndex++
			}
		case "WRITE":
			err = m.write(stdout, separator)
			separator = ";"
		case "CONST":
			m.stack = append(m.stack, inst.value)
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
			return fmt.Errorf("instruction %d (%s): %w", m.isntrucionPointer, inst.opcode, err)
		}
		m.isntrucionPointer++
	}
	return nil
}

func (m *StackMachine) pop() (int32, error) {
	if len(m.stack) == 0 {
		return 0, fmt.Errorf("stack underflow")
	}
	index := len(m.stack) - 1
	value := m.stack[index]
	m.stack = m.stack[:index]
	return value, nil
}

func (m *StackMachine) load(name string) error {
	value, exists := m.memory[name]
	if !exists {
		return fmt.Errorf("unknown variable %q", name)
	}
	m.stack = append(m.stack, value)
	return nil
}

func (m *StackMachine) store(name string) error {
	value, err := m.pop()
	if err != nil {
		return err
	}
	m.memory[name] = value
	return nil
}

func (m *StackMachine) binary(operator string) error {
	if len(m.stack) < 2 {
		return fmt.Errorf("stack underflow: binary operator requires two values")
	}
	index := len(m.stack) - 2
	value, err := calculate(operator, m.stack[index], m.stack[index+1])
	if err != nil {
		return err
	}
	m.stack = append(m.stack[:index], value)
	return nil
}

func (m *StackMachine) jump(opcode, label string) error {
	if opcode != "JMP" {
		condition, err := m.pop()
		if err != nil {
			return err
		}
		if opcode == "JZ" && condition != 0 || opcode == "JNZ" && condition == 0 {
			return nil
		}
	}
	m.isntrucionPointer = m.labels[label]
	return nil
}

func (m *StackMachine) write(stdout io.Writer, separator string) error {
	value, err := m.pop()
	if err != nil {
		return err
	}
	if err := writeText(stdout, separator+strconv.FormatInt(int64(value), 10)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func writeText(writer io.Writer, text string) error {
	n, err := io.WriteString(writer, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
