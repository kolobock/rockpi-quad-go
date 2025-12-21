package button

import (
	"context"
	"testing"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/config"
)

func TestEventType(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		wantStr   string
	}{
		{"click event", Click, "click"},
		{"double click event", DoubleClick, "twice"},
		{"long press event", LongPress, "press"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.eventType) != tt.wantStr {
				t.Errorf("EventType = %v, want %v", tt.eventType, tt.wantStr)
			}
		})
	}
}

func TestControllerCreation(t *testing.T) {
	cfg := &config.Config{
		Env: config.EnvConfig{
			ButtonChip: "",
			ButtonLine: "",
		},
		Time: config.TimeConfig{
			Twice: 0.7,
			Press: 1.8,
		},
	}

	ctrl, err := New(cfg)
	if err != nil {
		// Expected to fail when no GPIO pin is configured
		if cfg.Env.ButtonLine == "" {
			t.Skip("Button monitoring disabled - no pin configured")
		}
		t.Fatalf("New failed: %v", err)
	}

	if ctrl.pressChan == nil {
		t.Error("pressChan is nil")
	}
	if ctrl.twiceWindow != time.Duration(0.7*float64(time.Second)) {
		t.Errorf("twiceWindow = %v, want %v", ctrl.twiceWindow, time.Duration(0.7*float64(time.Second)))
	}
	if ctrl.pressTime != time.Duration(1.8*float64(time.Second)) {
		t.Errorf("pressTime = %v, want %v", ctrl.pressTime, time.Duration(1.8*float64(time.Second)))
	}
}

func TestPressChan(t *testing.T) {
	ctrl := &Controller{
		pressChan: make(chan EventType, 10),
	}

	ch := ctrl.PressChan()
	if ch == nil {
		t.Error("PressChan returned nil")
	}

	go func() {
		ctrl.pressChan <- Click
	}()

	select {
	case evt := <-ch:
		if evt != Click {
			t.Errorf("received event = %v, want %v", evt, Click)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}
func TestRunWithContextCancellation(t *testing.T) {
	// This test verifies that the controller's Run method properly handles
	// context cancellation and that the pressChan remains open and functional
	// for the duration of Run - regression test for defer Close() being
	// called too early which closed GPIO resources prematurely

	ctrl := &Controller{
		pressChan:   make(chan EventType, 10),
		eventChan:   nil, // No GPIO events in unit test
		twiceWindow: 700 * time.Millisecond,
		pressTime:   1800 * time.Millisecond,
		line:        nil, // No actual GPIO line for unit test
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in a goroutine (simulating actual usage)
	runComplete := make(chan struct{})
	go func() {
		defer close(runComplete)
		ctrl.Run(ctx)
	}()

	// Verify pressChan is still open and accessible
	select {
	case <-ctrl.PressChan():
		// Expected to block since no events sent yet
		t.Error("pressChan should block when no events")
	case <-time.After(50 * time.Millisecond):
		// Expected path - channel is open but has no data
	}

	// Cancel context
	cancel()

	// Wait for Run to complete
	select {
	case <-runComplete:
		// Expected - Run should exit when context is canceled
	case <-time.After(500 * time.Millisecond):
		t.Error("Run did not exit after context cancellation")
	}

	// Verify pressChan is still usable after Run exits
	// (it shouldn't be closed, just no longer receiving events)
	select {
	case <-ctrl.PressChan():
		// Should still block since no events
		t.Error("pressChan should still block after Run exits")
	case <-time.After(50 * time.Millisecond):
		// Expected - channel is still open
	}
}

func TestControllerLifecycle(t *testing.T) {
	// Test the complete lifecycle: create, run, cancel context, close
	// This simulates the actual usage pattern in main.go

	ctrl := &Controller{
		pressChan:   make(chan EventType, 10),
		eventChan:   nil, // No GPIO events in unit test
		twiceWindow: 700 * time.Millisecond,
		pressTime:   1800 * time.Millisecond,
		line:        nil, // No actual GPIO line for unit test
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start Run in a goroutine with deferred Close (like in main.go)
	runComplete := make(chan struct{})
	go func() {
		defer close(runComplete)
		defer func() {
			if err := ctrl.Close(); err != nil {
				t.Errorf("Close() error: %v", err)
			}
		}()
		ctrl.Run(ctx)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Verify channel is operational
	ch := ctrl.PressChan()
	if ch == nil {
		t.Error("PressChan returned nil during Run")
	}

	// Cancel and wait for completion
	cancel()

	select {
	case <-runComplete:
		// Expected - goroutine completed with deferred Close
	case <-time.After(500 * time.Millisecond):
		t.Error("Goroutine did not complete after context cancellation")
	}
}
