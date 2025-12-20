package main

import (
	"testing"

	"github.com/kolobock/rockpi-quad-go/internal/button"
	"github.com/kolobock/rockpi-quad-go/internal/config"
)

// Note: This test file can run without hardware dependencies
// since it only tests the getButtonAction function which has no hardware deps

func TestGetButtonAction(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Config
		event button.EventType
		want  string
	}{
		{
			name: "click event returns click action",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "slider",
					Twice: "switch",
					Press: "poweroff",
				},
			},
			event: button.Click,
			want:  "slider",
		},
		{
			name: "double click event returns twice action",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "slider",
					Twice: "switch",
					Press: "poweroff",
				},
			},
			event: button.DoubleClick,
			want:  "switch",
		},
		{
			name: "long press event returns press action",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "slider",
					Twice: "switch",
					Press: "poweroff",
				},
			},
			event: button.LongPress,
			want:  "poweroff",
		},
		{
			name: "unknown event returns none",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "slider",
					Twice: "switch",
					Press: "poweroff",
				},
			},
			event: button.EventType("unknown"),
			want:  "none",
		},
		{
			name: "custom command for click",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "echo 'custom action'",
					Twice: "reboot",
					Press: "poweroff",
				},
			},
			event: button.Click,
			want:  "echo 'custom action'",
		},
		{
			name: "reboot action",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "slider",
					Twice: "reboot",
					Press: "poweroff",
				},
			},
			event: button.DoubleClick,
			want:  "reboot",
		},
		{
			name: "empty action returns empty string",
			cfg: &config.Config{
				Key: config.KeyConfig{
					Click: "",
					Twice: "",
					Press: "",
				},
			},
			event: button.Click,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getButtonAction(tt.cfg, tt.event)
			if got != tt.want {
				t.Errorf("getButtonAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetButtonAction_AllEventTypes(t *testing.T) {
	cfg := &config.Config{
		Key: config.KeyConfig{
			Click: "action_click",
			Twice: "action_twice",
			Press: "action_press",
		},
	}

	eventTests := []struct {
		event button.EventType
		want  string
	}{
		{button.Click, "action_click"},
		{button.DoubleClick, "action_twice"},
		{button.LongPress, "action_press"},
	}

	for _, tt := range eventTests {
		got := getButtonAction(cfg, tt.event)
		if got != tt.want {
			t.Errorf("getButtonAction(cfg, %v) = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestGetButtonAction_BuiltinActions(t *testing.T) {
	builtinActions := []string{"slider", "switch", "poweroff", "reboot", "none"}

	for _, action := range builtinActions {
		cfg := &config.Config{
			Key: config.KeyConfig{
				Click: action,
				Twice: action,
				Press: action,
			},
		}

		t.Run("action_"+action, func(t *testing.T) {
			got := getButtonAction(cfg, button.Click)
			if got != action {
				t.Errorf("getButtonAction() with action %q = %q, want %q", action, got, action)
			}
		})
	}
}
