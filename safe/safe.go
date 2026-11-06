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

// Recover logs a panic instead of letting it crash the program. It must be used with defer.
func Recover(job string) {
	if r := recover(); r != nil {
		log.Printf("PANIC in background job %q: %v\n%s", job, r, debug.Stack())
	}
}
