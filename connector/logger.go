package connector

type Logger interface {
	Printf(format string, args ...any)
	Errorf(format string, args ...any)
}
