package compiler

type expression interface{ expressionNode() }

type literalExpression struct{ value int32 }
type variableExpression struct{ name string }
type stringExpression struct{ text string }
type arrayExpression struct{ elements []expression }
type lengthExpression struct{ object expression }
type elementExpression struct{ object, index expression }
type binaryExpression struct {
	operator string
	left     expression
	right    expression
}

func (literalExpression) expressionNode()  {}
func (variableExpression) expressionNode() {}
func (stringExpression) expressionNode()   {}
func (arrayExpression) expressionNode()    {}
func (lengthExpression) expressionNode()   {}
func (elementExpression) expressionNode()  {}
func (binaryExpression) expressionNode()   {}

type statement interface{ statementNode() }

type readStatement struct{ name string }
type writeStatement struct{ value expression }
type assignmentStatement struct {
	name  string
	value expression
}
type setStatement struct{ object, index, value expression }
type whileStatement struct {
	condition expression
	body      statement
}
type doStatement struct {
	body      statement
	condition expression
}
type ifStatement struct {
	condition expression
	then      statement
	otherwise statement
}
type blockStatement struct{ statements []statement }
type skipStatement struct{}

func (readStatement) statementNode()       {}
func (writeStatement) statementNode()      {}
func (assignmentStatement) statementNode() {}
func (setStatement) statementNode()        {}
func (whileStatement) statementNode()      {}
func (doStatement) statementNode()         {}
func (ifStatement) statementNode()         {}
func (blockStatement) statementNode()      {}
func (skipStatement) statementNode()       {}
