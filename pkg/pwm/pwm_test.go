package pwm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPWMDutyCycleCalculation(t *testing.T) {
	p := &PWM{
		period:   40000,
		inversed: false,
	}

	tests := []struct {
		dutyCycle float64
		wantDuty  int64
	}{
		{0.0, 0},
		{0.25, 10000},
		{0.5, 20000},
		{0.75, 30000},
		{1.0, 40000},
	}

	for _, tt := range tests {
		duty := int64(float64(p.period) * tt.dutyCycle)
		if duty != tt.wantDuty {
			t.Errorf("duty cycle %v: got %v, want %v", tt.dutyCycle, duty, tt.wantDuty)
		}
	}
}

func TestPWMWriteSysfs(t *testing.T) {
	tmpDir := t.TempDir()
	p := &PWM{
		basePath: tmpDir,
	}

	testFile := "test_value"
	testValue := "12345"

	err := p.writeSysfs(testFile, testValue)
	if err != nil {
		t.Fatalf("writeSysfs failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, testFile))
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if string(content) != testValue {
		t.Errorf("writeSysfs wrote %q, want %q", string(content), testValue)
	}
}
