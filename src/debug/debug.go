package debug

import (
	"fmt"
	"os"
	"sync/atomic"
)

var enabled atomic.Bool

func SetEnabled(value bool) {
	enabled.Store(value)
}

func Enabled() bool {
	return enabled.Load()
}

func Printf(format string, args ...any) {
	if enabled.Load() {
		fmt.Fprintf(os.Stderr, "[debug] "+format, args...)
	}
}

func Println(args ...any) {
	if enabled.Load() {
		fmt.Fprint(os.Stderr, "[debug] ")
		fmt.Fprintln(os.Stderr, args...)
	}
}
