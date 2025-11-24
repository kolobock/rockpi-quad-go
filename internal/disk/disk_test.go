package disk

import (
	"testing"
)

func TestGetTemperatureInvalidDevice(t *testing.T) {
	_, err := GetTemperature("/dev/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device, got nil")
	}
}

func TestEnableSATAControllerNoConfig(t *testing.T) {
	EnableSATAController("", "", "")
}
