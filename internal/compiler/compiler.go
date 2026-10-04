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
	if err := generator.compileBlock(program); err != nil {
		return nil, err
	}
	result, err := json.MarshalIndent(generator.instructions, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode program: %w", err)
	}
	return append(result, '\n'), nil
}
