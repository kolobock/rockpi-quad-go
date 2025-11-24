package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	configContent := `[fan]
lv0 = 35
lv1 = 40
lv2 = 45
lv3 = 50
max_cpu_temp = 80.0
max_disk_temp = 70.0

[oled]
rotate = false
f-temp = false

[disk]
space_usage_mnt_points = /|/mnt/disk1
disks_temp = /dev/sda

[network]
interfaces = eth0

[key]
click = slider
twice = switch
press = poweroff

[time]
twice = 0.7
press = 1.8
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Fan.LV0 != 35 {
		t.Errorf("Fan.LV0 = %v, want 35", cfg.Fan.LV0)
	}
	if cfg.Fan.MaxCPUTemp != 80.0 {
		t.Errorf("Fan.MaxCPUTemp = %v, want 80.0", cfg.Fan.MaxCPUTemp)
	}

	if cfg.Key.Click != "slider" {
		t.Errorf("Key.Click = %v, want slider", cfg.Key.Click)
	}

	if cfg.Time.Twice != 0.7 {
		t.Errorf("Time.Twice = %v, want 0.7", cfg.Time.Twice)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	configContent := `[fan]
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_defaults.conf")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Fan.LV0 != 35 {
		t.Errorf("default Fan.LV0 = %v, want 35", cfg.Fan.LV0)
	}
	if cfg.Time.Press != 1.8 {
		t.Errorf("default Time.Press = %v, want 1.8", cfg.Time.Press)
	}
}
