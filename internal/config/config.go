package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

type Config struct {
	Fan  FanConfig
	OLED OLEDConfig
	Disk DiskConfig
	Key  KeyConfig
}

type FanConfig struct {
	// Temperature levels (Celsius)
	LV0, LV1, LV2, LV3       float64
	LV0C, LV1C, LV2C, LV3C   float64 // CPU fan levels
	LV0F, LV1F, LV2F, LV3F   float64 // Disk fan levels
	MaxCPUTemp, MaxDiskTemp  float64

	Linear    bool
	TempDisks bool
	Syslog    bool

	// PWM configuration from environment
	CPUPWMChip    string
	CPUPWMChannel int
	TBPWMChip     string
	TBPWMChannel  int
	HardwarePWM   bool
	Polarity      string
}

type OLEDConfig struct {
	Enabled    bool
	Rotate     bool
	Fahrenheit bool
}

type DiskConfig struct {
	SpaceUsageMountPoints []string
	IOUsageMountPoints    []string
	TempDisks             []string
}

type KeyConfig struct {
	Click string
	Twice string
	Press string
}

func Load(path string) (*Config, error) {
	cfg := &Config{}

	iniFile, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Load fan configuration
	fanSec := iniFile.Section("fan")
	cfg.Fan.LV0 = fanSec.Key("lv0").MustFloat64(35)
	cfg.Fan.LV1 = fanSec.Key("lv1").MustFloat64(40)
	cfg.Fan.LV2 = fanSec.Key("lv2").MustFloat64(45)
	cfg.Fan.LV3 = fanSec.Key("lv3").MustFloat64(50)

	// CPU fan levels (fallback to general levels)
	cfg.Fan.LV0C = fanSec.Key("lv0c").MustFloat64(cfg.Fan.LV0)
	cfg.Fan.LV1C = fanSec.Key("lv1c").MustFloat64(cfg.Fan.LV1)
	cfg.Fan.LV2C = fanSec.Key("lv2c").MustFloat64(cfg.Fan.LV2)
	cfg.Fan.LV3C = fanSec.Key("lv3c").MustFloat64(cfg.Fan.LV3)

	// Disk fan levels (fallback to general levels)
	cfg.Fan.LV0F = fanSec.Key("lv0f").MustFloat64(cfg.Fan.LV0)
	cfg.Fan.LV1F = fanSec.Key("lv1f").MustFloat64(cfg.Fan.LV1)
	cfg.Fan.LV2F = fanSec.Key("lv2f").MustFloat64(cfg.Fan.LV2)
	cfg.Fan.LV3F = fanSec.Key("lv3f").MustFloat64(cfg.Fan.LV3)

	cfg.Fan.MaxCPUTemp = fanSec.Key("max_cpu_temp").MustFloat64(80.0)
	cfg.Fan.MaxDiskTemp = fanSec.Key("max_disk_temp").MustFloat64(70.0)

	cfg.Fan.Linear = fanSec.Key("linear").MustBool(false)
	cfg.Fan.TempDisks = fanSec.Key("temp_disks").MustBool(false)
	cfg.Fan.Syslog = fanSec.Key("syslog").MustBool(false)

	// Load environment variables for PWM
	cfg.Fan.HardwarePWM = os.Getenv("HARDWARE_PWM") == "1"
	cfg.Fan.CPUPWMChip = os.Getenv("PWM_CHIP")
	if cfg.Fan.CPUPWMChip == "" {
		cfg.Fan.CPUPWMChip = "pwmchip0"
	}
	cfg.Fan.CPUPWMChannel, _ = strconv.Atoi(os.Getenv("PWM_CPU_FAN"))
	cfg.Fan.TBPWMChannel, _ = strconv.Atoi(os.Getenv("PWM_TB_FAN"))
	if cfg.Fan.TBPWMChannel == 0 {
		cfg.Fan.TBPWMChannel = cfg.Fan.CPUPWMChannel
	}
	cfg.Fan.TBPWMChip = cfg.Fan.CPUPWMChip
	cfg.Fan.Polarity = os.Getenv("POLARITY")

	// Load OLED configuration
	oledSec := iniFile.Section("oled")
	cfg.OLED.Enabled = true
	cfg.OLED.Rotate = oledSec.Key("rotate").MustBool(false)
	cfg.OLED.Fahrenheit = oledSec.Key("f-temp").MustBool(false)

	// Load disk configuration
	diskSec := iniFile.Section("disk")
	if mountPoints := diskSec.Key("space_usage_mnt_points").String(); mountPoints != "" {
		cfg.Disk.SpaceUsageMountPoints = strings.Split(mountPoints, "|")
	}
	if ioPoints := diskSec.Key("io_usage_mnt_points").String(); ioPoints != "" {
		cfg.Disk.IOUsageMountPoints = strings.Split(ioPoints, "|")
	}
	if tempDisks := diskSec.Key("disks_temp").String(); tempDisks != "" {
		cfg.Disk.TempDisks = strings.Split(tempDisks, ",")
	}

	// Load key configuration
	keySec := iniFile.Section("key")
	cfg.Key.Click = keySec.Key("click").MustString("slider")
	cfg.Key.Twice = keySec.Key("twice").MustString("switch")
	cfg.Key.Press = keySec.Key("press").MustString("poweroff")

	return cfg, nil
}
