package log

import (
	"fmt"
	"os"
)

const (
	Red       = "\x1b[31m"
	Green     = "\x1b[32m"
	Yellow    = "\x1b[33m"
	Blue      = "\x1b[34m"
	Magenta   = "\x1b[35m"
	Cyan      = "\x1b[36m"
	Bold      = "\x1b[1m"
	Underline = "\x1b[4m"
	Reset     = "\x1b[0m"
)

var NoColor = false

func Fatal(v ...any) {
	Error(v...)
	os.Exit(1)
}

func Error(v ...any) {
	prefix := "error: "
	if !NoColor {
		prefix = fmt.Sprintf("%s%serror:%s ", Red, Bold, Reset)
	}
	fmt.Fprintf(os.Stderr, "%s %v", prefix, v)
}
