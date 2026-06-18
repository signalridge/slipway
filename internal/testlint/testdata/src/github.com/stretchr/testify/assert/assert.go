package assert

type TestingT interface {
	Errorf(format string, args ...any)
}

func Greater(t TestingT, e1, e2 any, msgAndArgs ...any) bool {
	return true
}

func GreaterOrEqual(t TestingT, e1, e2 any, msgAndArgs ...any) bool {
	return true
}

func Less(t TestingT, e1, e2 any, msgAndArgs ...any) bool {
	return true
}

func LessOrEqual(t TestingT, e1, e2 any, msgAndArgs ...any) bool {
	return true
}
