package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

type Config struct {
	Fan     FanConfig
	OLED    OLEDConfig
	Disk    DiskConfig
	Network NetworkConfig
	Key     KeyConfig
	Time    TimeConfig
	Env     EnvConfig
}

type EnvConfig struct {
	SDA         string
	SCL         string
	OLEDReset   string
	ButtonChip  string
	ButtonLine  string
	FanChip     string
	FanLine     string
	HardwarePWM string
	SATAChip    string
	SATALine1   string
	SATALine2   string
}

type FanConfig struct {
	// Temperature levels (Celsius)
	LV0, LV1, LV2, LV3      float64
	LV0C, LV1C, LV2C, LV3C  float64 // CPU fan levels
	LV0F, LV1F, LV2F, LV3F  float64 // Disk fan levels
	MaxCPUTemp, MaxDiskTemp float64

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

type NetworkConfig struct {
	Interfaces []string
}

type KeyConfig struct {
	Click string
	Twice string
	Press string
}

type TimeConfig struct {
	Twice float64 // seconds for double-click detection
	Press float64 // seconds for long-press detection
}

func Load(path string) (*Config, error) {
	cfg := &Config{}

	cfg.Env.SDA = os.Getenv("SDA")
	cfg.Env.SCL = os.Getenv("SCL")
	cfg.Env.OLEDReset = os.Getenv("OLED_RESET")
	cfg.Env.ButtonChip = os.Getenv("BUTTON_CHIP")
	cfg.Env.ButtonLine = os.Getenv("BUTTON_LINE")
	cfg.Env.FanChip = os.Getenv("FAN_CHIP")
	cfg.Env.FanLine = os.Getenv("FAN_LINE")
	cfg.Env.HardwarePWM = os.Getenv("HARDWARE_PWM")
	cfg.Env.SATAChip = os.Getenv("SATA_CHIP")
	cfg.Env.SATALine1 = os.Getenv("SATA_LINE_1")
	cfg.Env.SATALine2 = os.Getenv("SATA_LINE_2")

	iniFile, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	fanSec := iniFile.Section("fan")
	cfg.Fan.LV0 = fanSec.Key("lv0").MustFloat64(35)
	cfg.Fan.LV1 = fanSec.Key("lv1").MustFloat64(40)
	cfg.Fan.LV2 = fanSec.Key("lv2").MustFloat64(45)
	cfg.Fan.LV3 = fanSec.Key("lv3").MustFloat64(50)

	cfg.Fan.LV0C = fanSec.Key("lv0c").MustFloat64(cfg.Fan.LV0)
	cfg.Fan.LV1C = fanSec.Key("lv1c").MustFloat64(cfg.Fan.LV1)
	cfg.Fan.LV2C = fanSec.Key("lv2c").MustFloat64(cfg.Fan.LV2)
	cfg.Fan.LV3C = fanSec.Key("lv3c").MustFloat64(cfg.Fan.LV3)

	cfg.Fan.LV0F = fanSec.Key("lv0f").MustFloat64(cfg.Fan.LV0)
	cfg.Fan.LV1F = fanSec.Key("lv1f").MustFloat64(cfg.Fan.LV1)
	cfg.Fan.LV2F = fanSec.Key("lv2f").MustFloat64(cfg.Fan.LV2)
	cfg.Fan.LV3F = fanSec.Key("lv3f").MustFloat64(cfg.Fan.LV3)

	cfg.Fan.MaxCPUTemp = fanSec.Key("max_cpu_temp").MustFloat64(80.0)
	cfg.Fan.MaxDiskTemp = fanSec.Key("max_disk_temp").MustFloat64(70.0)

	cfg.Fan.Linear = fanSec.Key("linear").MustBool(false)
	cfg.Fan.TempDisks = fanSec.Key("temp_disks").MustBool(false)
	cfg.Fan.Syslog = fanSec.Key("syslog").MustBool(false)

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

	oledSec := iniFile.Section("oled")
	cfg.OLED.Enabled = true
	cfg.OLED.Rotate = oledSec.Key("rotate").MustBool(false)
	cfg.OLED.Fahrenheit = oledSec.Key("f-temp").MustBool(false)

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

	netSec := iniFile.Section("network")
	if interfaces := netSec.Key("interfaces").String(); interfaces != "" {
		cfg.Network.Interfaces = strings.Split(interfaces, ",")
	}

	keySec := iniFile.Section("key")
	cfg.Key.Click = keySec.Key("click").MustString("slider")
	cfg.Key.Twice = keySec.Key("twice").MustString("switch")
	cfg.Key.Press = keySec.Key("press").MustString("poweroff")

	timeSec := iniFile.Section("time")
	cfg.Time.Twice = timeSec.Key("twice").MustFloat64(0.7)
	cfg.Time.Press = timeSec.Key("press").MustFloat64(1.8)

	return cfg, nil
}
