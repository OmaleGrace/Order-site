// Package safe stops one failing background job from taking the whole site down.
//
// Go only recovers a panic inside the goroutine that serves a web request. A panic in a
// goroutine started with "go ..." (an email, a text message, a notification) is not recovered
// and ends the entire program. Put this first inside such a job:
//
//	defer safe.Recover("what this job does")
package safe

import (
	"log"
	"runtime/debug"
)

// OnPanic, when set, is told about every panic that Recover swallows (main uses it to email an
// alert). It must not panic itself and should return quickly.
var OnPanic func(job string, value any, stack []byte)

// Recover logs a panic instead of letting it crash the program. It must be used with defer.
func Recover(job string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		log.Printf("PANIC in background job %q: %v\n%s", job, r, stack)
		if hook := OnPanic; hook != nil {
			func() {
				defer func() { _ = recover() }()
				hook(job, r, stack)
			}()
		}
	}
}
