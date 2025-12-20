package logger

import (
	"log"
	"sync"
)

var (
	verboseLogging bool
	mu             sync.RWMutex
)

func init() {
	log.SetFlags(0)
}

// SetVerbose enables or disables info/debug logging
func SetVerbose(enabled bool) {
	mu.Lock()
	verboseLogging = enabled
	mu.Unlock()
}

// Info logs informational messages only if verbose logging is enabled
func Info(format string, v ...any) {
	mu.RLock()
	verbose := verboseLogging
	mu.RUnlock()

	if verbose {
		log.Printf(format, v...)
	}
}

// Infof is an alias for Info
func Infof(format string, v ...any) {
	Info(format, v...)
}

// Infoln logs informational messages only if verbose logging is enabled
func Infoln(v ...any) {
	mu.RLock()
	verbose := verboseLogging
	mu.RUnlock()

	if verbose {
		log.Println(v...)
	}
}

// Error logs error messages (always logged)
func Error(format string, v ...any) {
	log.Printf(format, v...)
}

// Errorf is an alias for Error
func Errorf(format string, v ...any) {
	log.Printf(format, v...)
}

// Fatal logs fatal messages and exits (always logged)
func Fatal(format string, v ...any) {
	log.Fatalf(format, v...)
}

// Fatalf is an alias for Fatal
func Fatalf(format string, v ...any) {
	log.Fatalf(format, v...)
}
