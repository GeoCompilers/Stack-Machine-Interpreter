package compiler

import "fmt"

type generator struct {
	instructions []any
	nextLabel    int
}

func (g *generator) emit(opcode string) {
	g.instructions = append(g.instructions, opcode)
}

func (g *generator) emitString(opcode string, operand string) {
	g.instructions = append(g.instructions, map[string]string{opcode: operand})
}

func (g *generator) emitConst(value int32) {
	g.instructions = append(g.instructions, map[string]int32{"CONST": value})
}

func (g *generator) label(prefix string) string {
	label := fmt.Sprintf("L_%s_%d", prefix, g.nextLabel)
	g.nextLabel++
	return label
}

func (g *generator) compileBlock(block blockStatement) error {
	for _, statement := range block.statements {
		if err := g.compileStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) compileStatement(node statement) error {
	switch value := node.(type) {
	case readStatement:
		g.emit("READ")
		g.emitString("ST", value.name)
	case writeStatement:
		if err := g.compileExpression(value.value); err != nil {
			return err
		}
		g.emit("WRITE")
	case assignmentStatement:
		if value.operator != "" {
			g.emitString("LD", value.name)
		}
		if err := g.compileExpression(value.value); err != nil {
			return err
		}
		if value.operator != "" {
			g.emitString("BINOP", value.operator)
		}
		g.emitString("ST", value.name)
	case whileStatement:
		return g.compileWhile(value.condition, value.body)
	case doStatement:
		bodyLabel := g.label("while_body")
		conditionLabel := g.label("while_cond")
		g.emitString("LABEL", bodyLabel)
		if err := g.compileStatement(value.body); err != nil {
			return err
		}
		g.emitString("LABEL", conditionLabel)
		if err := g.compileExpression(value.condition); err != nil {
			return err
		}
		g.emitString("JNZ", bodyLabel)
	case forStatement:
		if err := g.compileStatement(value.initial); err != nil {
			return err
		}
		return g.compileWhile(value.condition, blockStatement{statements: []statement{value.body, value.post}})
	case ifStatement:
		return g.compileIf(value)
	case blockStatement:
		return g.compileBlock(value)
	case skipStatement:
	default:
		return fmt.Errorf("internal compiler error: unsupported statement %T", node)
	}
	return nil
}

func (g *generator) compileWhile(condition expression, body statement) error {
	bodyLabel := g.label("while_body")
	conditionLabel := g.label("while_cond")
	g.emitString("JMP", conditionLabel)
	g.emitString("LABEL", bodyLabel)
	if err := g.compileStatement(body); err != nil {
		return err
	}
	g.emitString("LABEL", conditionLabel)
	if err := g.compileExpression(condition); err != nil {
		return err
	}
	g.emitString("JNZ", bodyLabel)
	return nil
}

func (g *generator) compileIf(value ifStatement) error {
	elseLabel := g.label("else")
	endLabel := g.label("end")
	if err := g.compileExpression(value.condition); err != nil {
		return err
	}
	g.emitString("JZ", elseLabel)
	if err := g.compileStatement(value.then); err != nil {
		return err
	}
	g.emitString("JMP", endLabel)
	g.emitString("LABEL", elseLabel)
	if value.otherwise != nil {
		if err := g.compileStatement(value.otherwise); err != nil {
			return err
		}
	}
	g.emitString("LABEL", endLabel)
	return nil
}

func (g *generator) compileExpression(node expression) error {
	switch value := node.(type) {
	case literalExpression:
		g.emitConst(value.value)
	case variableExpression:
		g.emitString("LD", value.name)
	case binaryExpression:
		if err := g.compileExpression(value.left); err != nil {
			return err
		}
		if err := g.compileExpression(value.right); err != nil {
			return err
		}
		g.emitString("BINOP", value.operator)
	default:
		return fmt.Errorf("internal compiler error: unsupported expression %T", node)
	}
	return nil
}
