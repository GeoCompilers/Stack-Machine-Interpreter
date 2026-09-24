package stackmachine

import "fmt"

func validOperator(operator string) bool {
	switch operator {
	case "+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "!!":
		return true
	default:
		return false
	}
}

func calculate(operator string, left, right int32) (int32, error) {
	switch operator {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/", "%":
		if right == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		if operator == "/" {
			return left / right, nil
		}
		return left % right, nil
	case "==":
		return boolean(left == right), nil
	case "!=":
		return boolean(left != right), nil
	case "<":
		return boolean(left < right), nil
	case "<=":
		return boolean(left <= right), nil
	case ">":
		return boolean(left > right), nil
	case ">=":
		return boolean(left >= right), nil
	case "&&":
		return boolean(left != 0 && right != 0), nil
	case "!!":
		return boolean(left != 0 || right != 0), nil
	default:
		return 0, fmt.Errorf("unknown operator %q", operator)
	}
}

func boolean(value bool) int32 {
	if value {
		return 1
	}
	return 0
}
