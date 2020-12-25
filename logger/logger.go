// Package logger provides a simple log interface that you can wrap whatever
// logging library you use into.
package logger

import (
	"log"
	"os"
)

// Logger is a simple common ground to wrap whatever logging library the user
// prefers into.
type Logger func(msg string)

// Log is the nil-safe way of using a Logger.
//
// If l is nil it does nothing. Otherwise it calls l.
func (l Logger) Log(msg string) {
	if l != nil {
		l(msg)
	}
}

// NopLogger is a Logger implementation that does nothing.
func NopLogger(_ string) {}

var stdFallback = log.New(os.Stderr, "", log.LstdFlags)

// StdLogger is a Logger implementation that wraps stdlib log package.
//
// If passed in logger is nil, it falls back to
//
//     log.New(os.Stderr, "", log.LstdFlags)
func StdLogger(logger *log.Logger) Logger {
	if logger == nil {
		logger = stdFallback
	}
	return func(msg string) {
		logger.Print(msg)
	}
}
