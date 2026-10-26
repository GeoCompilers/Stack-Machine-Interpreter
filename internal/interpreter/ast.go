package interpreter

type expression struct {
	kind, text  string
	number      int32
	left, right *expression
	elements    []*expression
}

type statement struct {
	kind, name                      string
	value, index, object, condition *expression
	first, second, otherwise        *statement
}
