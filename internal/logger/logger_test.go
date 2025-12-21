package logger

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// captureOutput captures log output for testing
func captureOutput(f func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	return buf.String()
}

func TestSetVerbose(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enable verbose", true},
		{"disable verbose", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVerbose(tt.enabled)
			mu.RLock()
			got := verboseLogging
			mu.RUnlock()
			if got != tt.enabled {
				t.Errorf("SetVerbose(%v) = %v, want %v", tt.enabled, got, tt.enabled)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		format   string
		args     []any
		wantLogs bool
	}{
		{
			name:     "logs when verbose enabled",
			verbose:  true,
			format:   "test message: %s",
			args:     []any{"hello"},
			wantLogs: true,
		},
		{
			name:     "no logs when verbose disabled",
			verbose:  false,
			format:   "test message: %s",
			args:     []any{"hello"},
			wantLogs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVerbose(tt.verbose)
			output := captureOutput(func() {
				Infof(tt.format, tt.args...)
			})

			if tt.wantLogs {
				if output == "" {
					t.Errorf("Info() produced no output, expected logs")
				}
				if !strings.Contains(output, "hello") {
					t.Errorf("Info() output = %q, want to contain %q", output, "hello")
				}
			} else if output != "" {
				t.Errorf("Info() produced output %q, expected no logs", output)
			}
		})
	}
}

func TestInfof(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		format   string
		args     []any
		wantLogs bool
	}{
		{
			name:     "logs when verbose enabled",
			verbose:  true,
			format:   "formatted: %d %s",
			args:     []any{42, "test"},
			wantLogs: true,
		},
		{
			name:     "no logs when verbose disabled",
			verbose:  false,
			format:   "formatted: %d %s",
			args:     []any{42, "test"},
			wantLogs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVerbose(tt.verbose)
			output := captureOutput(func() {
				Infof(tt.format, tt.args...)
			})

			if tt.wantLogs {
				if output == "" {
					t.Errorf("Infof() produced no output, expected logs")
				}
				if !strings.Contains(output, "42") || !strings.Contains(output, "test") {
					t.Errorf("Infof() output = %q, want to contain formatted message", output)
				}
			} else if output != "" {
				t.Errorf("Infof() produced output %q, expected no logs", output)
			}
		})
	}
}

func TestInfoln(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		args     []any
		wantLogs bool
	}{
		{
			name:     "logs when verbose enabled",
			verbose:  true,
			args:     []any{"test", "message", 123},
			wantLogs: true,
		},
		{
			name:     "no logs when verbose disabled",
			verbose:  false,
			args:     []any{"test", "message", 123},
			wantLogs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVerbose(tt.verbose)
			output := captureOutput(func() {
				Infoln(tt.args...)
			})

			if tt.wantLogs {
				if output == "" {
					t.Errorf("Infoln() produced no output, expected logs")
				}
				if !strings.Contains(output, "test") {
					t.Errorf("Infoln() output = %q, want to contain %q", output, "test")
				}
			} else if output != "" {
				t.Errorf("Infoln() produced output %q, expected no logs", output)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "logs error message",
			format: "error: %s",
			args:   []any{"something failed"},
			want:   "something failed",
		},
		{
			name:   "logs multiple args",
			format: "error code %d: %s",
			args:   []any{500, "internal error"},
			want:   "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				Errorf(tt.format, tt.args...)
			})

			if !strings.Contains(output, tt.want) {
				t.Errorf("Error() output = %q, want to contain %q", output, tt.want)
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "logs error message",
			format: "error: %s",
			args:   []any{"test error"},
			want:   "test error",
		},
		{
			name:   "logs formatted message",
			format: "failed with code %d",
			args:   []any{404},
			want:   "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				Errorf(tt.format, tt.args...)
			})

			if !strings.Contains(output, tt.want) {
				t.Errorf("Errorf() output = %q, want to contain %q", output, tt.want)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test concurrent access to verbose flag
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(i int) {
			SetVerbose(i%2 == 0)
			Infof("test message %d", i)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
}
