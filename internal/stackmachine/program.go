package stackmachine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

type instruction struct {
	opcode  string
	operand string
	value   int32
}

func NewStackMachine(code []byte) (*StackMachine, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(code, &raw); err != nil {
		return nil, fmt.Errorf("invalid program: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("program must be a JSON array")
	}
	if len(raw) > math.MaxInt32 {
		return nil, fmt.Errorf("too many instructions")
	}
	machine := &StackMachine{
		memory:       make(map[string]int32),
		instructions: make([]instruction, len(raw)),
		labels:       make(map[string]int32),
	}
	for i, data := range raw {
		inst, err := parseInstruction(data)
		if err != nil {
			return nil, fmt.Errorf("instruction %d: %w", i, err)
		}
		machine.instructions[i] = inst
		if inst.opcode == "LABEL" {
			if _, exists := machine.labels[inst.operand]; exists {
				return nil, fmt.Errorf("instruction %d: duplicate label %q", i, inst.operand)
			}
			machine.labels[inst.operand] = int32(i)
		}
	}
	for i, inst := range machine.instructions {
		switch inst.opcode {
		case "JMP", "JZ", "JNZ":
			if _, exists := machine.labels[inst.operand]; !exists {
				return nil, fmt.Errorf("instruction %d: unknown label %q", i, inst.operand)
			}
		}
	}
	return machine, nil
}

func parseInstruction(data []byte) (instruction, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return instruction{}, err
	}
	if opcode, ok := token.(string); ok {
		if opcode != "READ" && opcode != "WRITE" {
			return instruction{}, fmt.Errorf("unknown instruction %q", opcode)
		}
		return instruction{opcode: opcode}, nil
	}
	if token != json.Delim('{') || !decoder.More() {
		return instruction{}, fmt.Errorf("expected READ, WRITE or a single-key object")
	}
	key, err := decoder.Token()
	if err != nil {
		return instruction{}, err
	}
	opcode, ok := key.(string)
	if !ok {
		return instruction{}, fmt.Errorf("expected instruction name")
	}
	var operand json.RawMessage
	if err := decoder.Decode(&operand); err != nil {
		return instruction{}, err
	}
	if decoder.More() {
		return instruction{}, fmt.Errorf("instruction must have exactly one key")
	}
	return parseOperand(opcode, operand)
}

func parseOperand(opcode string, data []byte) (instruction, error) {
	inst := instruction{opcode: opcode}
	if opcode == "CONST" {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, &inst.value) != nil {
			return instruction{}, fmt.Errorf("CONST requires an int32 JSON integer")
		}
		return inst, nil
	}
	switch opcode {
	case "LD", "ST", "LABEL", "JMP", "JZ", "JNZ", "BINOP":
	default:
		return instruction{}, fmt.Errorf("unknown instruction %q", opcode)
	}
	if err := json.Unmarshal(data, &inst.operand); err != nil || inst.operand == "" {
		return instruction{}, fmt.Errorf("%s requires a nonempty string", opcode)
	}
	if opcode == "BINOP" && !validOperator(inst.operand) {
		return instruction{}, fmt.Errorf("unknown operator %q", inst.operand)
	}
	return inst, nil
}
