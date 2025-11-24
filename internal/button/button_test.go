package button

import (
	"testing"
	"time"
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
	ctrl, err := New("", "", 0.7, 1.8)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if ctrl.pressChan == nil {
		t.Error("pressChan is nil")
	}
	if ctrl.twiceWindow != time.Duration(0.7*float64(time.Second)) {
		t.Errorf("twiceWindow = %v, want %v", ctrl.twiceWindow, time.Duration(0.7*float64(time.Second)))
	}
}
