package require

type TestingT interface {
	Errorf(format string, args ...any)
}

func Greater(t TestingT, e1, e2 any, msgAndArgs ...any) {
}

func GreaterOrEqual(t TestingT, e1, e2 any, msgAndArgs ...any) {
}

func Less(t TestingT, e1, e2 any, msgAndArgs ...any) {
}

func LessOrEqual(t TestingT, e1, e2 any, msgAndArgs ...any) {
}
