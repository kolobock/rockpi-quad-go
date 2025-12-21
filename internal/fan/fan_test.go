package fan

import (
	"testing"

	"github.com/kolobock/rockpi-quad-go/internal/config"
)

func TestCalculateDutyCycleNonLinear(t *testing.T) {
	cfg := &config.Config{
		Fan: config.FanConfig{
			LV0C:       35,
			LV1C:       40,
			LV2C:       45,
			LV3C:       50,
			MaxCPUTemp: 80,
			Linear:     false,
		},
	}

	ctrl := &Controller{cfg: cfg}

	tests := []struct {
		name     string
		temp     float64
		key      byte
		wantDuty float64
	}{
		{"cpu below lv0", 30, 'c', 0.0},
		{"cpu at lv0", 35, 'c', 0.25},
		{"cpu between lv0 and lv1", 37, 'c', 0.25},
		{"cpu at lv1", 40, 'c', 0.50},
		{"cpu between lv1 and lv2", 42, 'c', 0.50},
		{"cpu above lv3", 60, 'c', 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.calculateDutyCycle(tt.temp, tt.key)
			if got != tt.wantDuty {
				t.Errorf("calculateDutyCycle(%v, %c) = %v, want %v", tt.temp, tt.key, got, tt.wantDuty)
			}
		})
	}
}

func TestGetFanSpeeds(t *testing.T) {
	ctrl := &Controller{
		lastCPUDC:  0.5,
		lastDiskDC: 0.75,
	}

	cpuPercent, diskPercent := ctrl.GetFanSpeeds()

	if cpuPercent != 50.0 {
		t.Errorf("CPU fan speed = %v%%, want 50.0%%", cpuPercent)
	}
	if diskPercent != 75.0 {
		t.Errorf("Disk fan speed = %v%%, want 75.0%%", diskPercent)
	}
}
