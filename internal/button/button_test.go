package button

import (
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
