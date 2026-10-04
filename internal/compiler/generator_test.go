package compiler

import (
	"strings"
	"testing"
)

type unknownStatement struct{}
type unknownExpression struct{}

func (unknownStatement) statementNode()   {}
func (unknownExpression) expressionNode() {}

func TestGeneratorRejectsUnknownStatements(t *testing.T) {
	literal := literalExpression{value: 1}
	skip := skipStatement{}
	unknown := unknownStatement{}
	cases := []struct {
		name string
		node statement
		want string
	}{
		{"unknown", unknown, "statement compiler.unknownStatement"},
		{"nil", nil, "statement <nil>"},
		{"typed_nil", (*unknownStatement)(nil), "statement *compiler.unknownStatement"},
		{"nested_block", blockStatement{statements: []statement{skip, blockStatement{statements: []statement{unknown}}}}, "statement compiler.unknownStatement"},
		{"write", writeStatement{value: unknownExpression{}}, "expression compiler.unknownExpression"},
		{"assignment", assignmentStatement{name: "x", value: unknownExpression{}}, "expression compiler.unknownExpression"},
		{"while_body", whileStatement{condition: literal, body: unknown}, "statement compiler.unknownStatement"},
		{"while_condition", whileStatement{condition: unknownExpression{}, body: skip}, "expression compiler.unknownExpression"},
		{"do_body", doStatement{body: unknown, condition: literal}, "statement compiler.unknownStatement"},
		{"do_condition", doStatement{body: skip, condition: unknownExpression{}}, "expression compiler.unknownExpression"},
		{"for_initial", forStatement{initial: unknown, condition: literal, post: skip, body: skip}, "statement compiler.unknownStatement"},
		{"for_post", forStatement{initial: skip, condition: literal, post: unknown, body: skip}, "statement compiler.unknownStatement"},
		{"for_body", forStatement{initial: skip, condition: literal, post: skip, body: unknown}, "statement compiler.unknownStatement"},
		{"for_condition", forStatement{initial: skip, condition: unknownExpression{}, post: skip, body: skip}, "expression compiler.unknownExpression"},
		{"if_condition", ifStatement{condition: unknownExpression{}, then: skip}, "expression compiler.unknownExpression"},
		{"if_then", ifStatement{condition: literal, then: unknown}, "statement compiler.unknownStatement"},
		{"if_else", ifStatement{condition: literal, then: skip, otherwise: unknown}, "statement compiler.unknownStatement"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var generator generator
			err := generator.compileStatement(test.node)
			if err == nil || !strings.Contains(err.Error(), "internal compiler error") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected internal error naming %q, got %v", test.want, err)
			}
		})
	}
}

func TestGeneratorRejectsUnknownExpressions(t *testing.T) {
	cases := []struct {
		name string
		node expression
		want string
	}{
		{"unknown", unknownExpression{}, "compiler.unknownExpression"},
		{"nil", nil, "<nil>"},
		{"typed_nil", (*unknownExpression)(nil), "*compiler.unknownExpression"},
		{"left_operand", binaryExpression{operator: "+", left: unknownExpression{}, right: literalExpression{value: 1}}, "compiler.unknownExpression"},
		{"right_operand", binaryExpression{operator: "+", left: literalExpression{value: 1}, right: unknownExpression{}}, "compiler.unknownExpression"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var generator generator
			err := generator.compileExpression(test.node)
			if err == nil || !strings.Contains(err.Error(), "internal compiler error: unsupported expression "+test.want) {
				t.Fatalf("expected internal error naming %q, got %v", test.want, err)
			}
		})
	}
}
