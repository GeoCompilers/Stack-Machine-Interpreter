package compiler

import (
	"encoding/json"
	"fmt"
)

func Compile(programJSON []byte) ([]byte, error) {
	program, err := decodeStatement(programJSON)
	if err != nil {
		return nil, err
	}
	generator := generator{instructions: make([]any, 0)}
	endLabel := generator.label("end")
	used, err := generator.compileStatement(program, endLabel)
	if err != nil {
		return nil, err
	}
	if used {
		generator.emitString("LABEL", endLabel)
	}
	result, err := json.MarshalIndent(generator.instructions, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode program: %w", err)
	}
	return append(result, '\n'), nil
}
