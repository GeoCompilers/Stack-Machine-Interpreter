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

func (g *generator) compileBlock(block blockStatement, exitLabel string) (bool, error) {
	for index, statement := range block.statements {
		if index == len(block.statements)-1 {
			return g.compileStatement(statement, exitLabel)
		}
		nextLabel := g.label("next")
		used, err := g.compileStatement(statement, nextLabel)
		if err != nil {
			return false, err
		}
		if used {
			g.emitString("LABEL", nextLabel)
		}
	}
	return false, nil
}

func (g *generator) compileStatement(node statement, exitLabel string) (bool, error) {
	switch value := node.(type) {
	case readStatement:
		g.emit("READ")
		g.emitString("ST", value.name)
	case writeStatement:
		if err := g.compileExpression(value.value); err != nil {
			return false, err
		}
		g.emit("WRITE")
	case assignmentStatement:
		if value.operator != "" {
			g.emitString("LD", value.name)
		}
		if err := g.compileExpression(value.value); err != nil {
			return false, err
		}
		if value.operator != "" {
			g.emitString("BINOP", value.operator)
		}
		g.emitString("ST", value.name)
	case whileStatement:
		return false, g.compileLoop(value.condition, value.body, true)
	case doStatement:
		return false, g.compileLoop(value.condition, value.body, false)
	case forStatement:
		return g.compileBlock(blockStatement{statements: []statement{
			value.initial,
			whileStatement{
				condition: value.condition,
				body:      blockStatement{statements: []statement{value.body, value.post}},
			},
		}}, exitLabel)
	case ifStatement:
		return g.compileIf(value, exitLabel)
	case blockStatement:
		return g.compileBlock(value, exitLabel)
	case skipStatement:
	default:
		return false, fmt.Errorf("internal compiler error: unsupported statement %T", node)
	}
	return false, nil
}

func (g *generator) compileLoop(condition expression, body statement, checkFirst bool) error {
	bodyLabel := g.label("while_body")
	conditionLabel := g.label("while_cond")
	if checkFirst {
		g.emitString("JMP", conditionLabel)
	}
	g.emitString("LABEL", bodyLabel)
	used, err := g.compileStatement(body, conditionLabel)
	if err != nil {
		return err
	}
	if checkFirst || used {
		g.emitString("LABEL", conditionLabel)
	}
	if err := g.compileExpression(condition); err != nil {
		return err
	}
	g.emitString("JNZ", bodyLabel)
	return nil
}

func (g *generator) compileIf(value ifStatement, exitLabel string) (bool, error) {
	if err := g.compileExpression(value.condition); err != nil {
		return false, err
	}
	if value.otherwise == nil {
		g.emitString("JZ", exitLabel)
		if _, err := g.compileStatement(value.then, exitLabel); err != nil {
			return false, err
		}
		return true, nil
	}
	elseLabel := g.label("else")
	g.emitString("JZ", elseLabel)
	if _, err := g.compileStatement(value.then, exitLabel); err != nil {
		return false, err
	}
	g.emitString("JMP", exitLabel)
	g.emitString("LABEL", elseLabel)
	if _, err := g.compileStatement(value.otherwise, exitLabel); err != nil {
		return false, err
	}
	return true, nil
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
