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

// Infof logs informational messages only if verbose logging is enabled
func Infof(format string, v ...any) {
	mu.RLock()
	verbose := verboseLogging
	mu.RUnlock()

	if verbose {
		log.Printf(format, v...)
	}
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

// Errorf logs error messages (always logged)
func Errorf(format string, v ...any) {
	log.Printf(format, v...)
}

// Fatalf logs fatal messages and exits (always logged)
func Fatalf(format string, v ...any) {
	log.Fatalf(format, v...)
}
