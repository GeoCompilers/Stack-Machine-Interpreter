package compiler

import "fmt"

type generator struct {
	instructions []any
	nextLabel    int
}

func (g *generator) emit(opcode string) {
	g.instructions = append(g.instructions, opcode)
}

func (g *generator) emitOperand(opcode string, operand any) {
	g.instructions = append(g.instructions, map[string]any{opcode: operand})
}

func (g *generator) label(prefix string) string {
	label := fmt.Sprintf("L_%s_%d", prefix, g.nextLabel)
	g.nextLabel++
	return label
}

func (g *generator) compileBlock(block blockStatement) {
	for _, statement := range block.statements {
		g.compileStatement(statement)
	}
}

func (g *generator) compileStatement(node statement) {
	switch value := node.(type) {
	case readStatement:
		g.emit("READ")
		g.emitOperand("ST", value.name)
	case writeStatement:
		g.compileExpression(value.value)
		g.emit("WRITE")
	case assignmentStatement:
		if value.operator != "" {
			g.emitOperand("LD", value.name)
		}
		g.compileExpression(value.value)
		if value.operator != "" {
			g.emitOperand("BINOP", value.operator)
		}
		g.emitOperand("ST", value.name)
	case whileStatement:
		g.compileWhile(value.condition, value.body)
	case doStatement:
		bodyLabel := g.label("while_body")
		conditionLabel := g.label("while_cond")
		g.emitOperand("LABEL", bodyLabel)
		g.compileStatement(value.body)
		g.emitOperand("LABEL", conditionLabel)
		g.compileExpression(value.condition)
		g.emitOperand("JNZ", bodyLabel)
	case forStatement:
		g.compileStatement(value.initial)
		g.compileWhile(value.condition, blockStatement{statements: []statement{value.body, value.post}})
	case ifStatement:
		g.compileIf(value)
	case blockStatement:
		g.compileBlock(value)
	case skipStatement:
	}
}

func (g *generator) compileWhile(condition expression, body statement) {
	bodyLabel := g.label("while_body")
	conditionLabel := g.label("while_cond")
	g.emitOperand("JMP", conditionLabel)
	g.emitOperand("LABEL", bodyLabel)
	g.compileStatement(body)
	g.emitOperand("LABEL", conditionLabel)
	g.compileExpression(condition)
	g.emitOperand("JNZ", bodyLabel)
}

func (g *generator) compileIf(value ifStatement) {
	elseLabel := g.label("else")
	endLabel := g.label("end")
	g.compileExpression(value.condition)
	g.emitOperand("JZ", elseLabel)
	g.compileStatement(value.then)
	g.emitOperand("JMP", endLabel)
	g.emitOperand("LABEL", elseLabel)
	if value.otherwise != nil {
		g.compileStatement(value.otherwise)
	}
	g.emitOperand("LABEL", endLabel)
}

func (g *generator) compileExpression(expression expression) {
	switch value := expression.(type) {
	case literalExpression:
		g.emitOperand("CONST", value.value)
	case variableExpression:
		g.emitOperand("LD", value.name)
	case binaryExpression:
		g.compileExpression(value.left)
		g.compileExpression(value.right)
		g.emitOperand("BINOP", value.operator)
	}
}
