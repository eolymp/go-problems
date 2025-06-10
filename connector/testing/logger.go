package testing

import (
	"fmt"
	"testing"
)

type TestLogger struct {
	t *testing.T
}

func MockLogger(t *testing.T) *TestLogger {
	return &TestLogger{t: t}
}

func (l *TestLogger) Printf(format string, args ...any) {
	l.t.Log("logger.PRINT: " + fmt.Sprintf(format, args...))
}

func (l *TestLogger) Errorf(format string, args ...any) {
	l.t.Log("logger.ERROR: " + fmt.Sprintf(format, args...))
}
