package compiler

import (
	"encoding/json"
	"fmt"
)

func Compile(source string) ([]byte, error) {
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	program, err := newParser(tokens).parseProgram()
	if err != nil {
		return nil, err
	}
	generator := generator{instructions: make([]any, 0)}
	endLabel := generator.label("end")
	used, err := generator.compileBlock(program, endLabel)
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
