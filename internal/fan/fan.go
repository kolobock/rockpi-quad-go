package fan

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/disk"
	"github.com/kolobock/rockpi-quad-go/internal/logger"
	"github.com/kolobock/rockpi-quad-go/pkg/pwm"
)

const (
	MinDutyCycle = 0.05
)

type Controller struct {
	cfg     *config.Config
	cpuPWM  *pwm.PWM
	diskPWM *pwm.PWM

	lastCPUDC    float64
	lastDiskDC   float64
	lastTemp     time.Time
	lastDiskTemp float64
	enabled      bool
	mu           sync.Mutex
}

func New(cfg *config.Config) (*Controller, error) {
	ctrl := &Controller{
		cfg:      cfg,
		lastTemp: time.Now().Add(-time.Hour),
		enabled:  true,
	}

	cpuPWM, err := pwm.New(cfg.Fan.CPUPWMChip, cfg.Fan.CPUPWMChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to init CPU PWM: %w", err)
	}
	ctrl.cpuPWM = cpuPWM

	if cfg.Fan.Polarity == "inversed" {
		cpuPWM.SetInversed(true)
	}

	if cfg.Fan.TBPWMChannel != cfg.Fan.CPUPWMChannel {
		diskPWM, err := pwm.New(cfg.Fan.TBPWMChip, cfg.Fan.TBPWMChannel)
		if err != nil {
			cpuPWM.Close()
			return nil, fmt.Errorf("failed to init disk PWM: %w", err)
		}
		ctrl.diskPWM = diskPWM
		if cfg.Fan.Polarity == "inversed" {
			diskPWM.SetInversed(true)
		}
	}

	return ctrl, nil
}

// ToggleFan toggles fan control on/off
func (c *Controller) ToggleFan() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = !c.enabled

	if c.enabled {
		logger.Infoln("Fan control enabled - temperature-based control resumed")
	} else {
		fullSpeed := 100.0
		if c.cfg.Fan.Polarity == "inversed" {
			fullSpeed = 0.0
		}

		logger.Infof("Fan control disabled - setting fans to full speed (DC: %.0f%%)", fullSpeed)
		if c.cpuPWM != nil {
			c.cpuPWM.SetDutyCycle(fullSpeed)
			c.lastCPUDC = fullSpeed
		}
		if c.diskPWM != nil {
			c.diskPWM.SetDutyCycle(fullSpeed)
			c.lastDiskDC = fullSpeed
		}
	}
}

func (c *Controller) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.update(); err != nil {
				logger.Errorf("Fan update error: %v", err)
			}
		}
	}
}

func (c *Controller) update() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return nil
	}

	cpuTemp, diskTemp := c.getTemperatures()

	cpuDC := c.calculateDutyCycle(cpuTemp, 'c')
	diskDC := c.calculateDutyCycle(diskTemp, 'f')

	if cpuDC > 0 && cpuDC < MinDutyCycle {
		cpuDC = MinDutyCycle
	}
	if diskDC > 0 && diskDC < MinDutyCycle {
		diskDC = MinDutyCycle
	}

	if cpuDC != c.lastCPUDC {
		if err := c.cpuPWM.SetDutyCycle(cpuDC); err != nil {
			return err
		}
		c.lastCPUDC = cpuDC
	}

	if c.diskPWM != nil {
		if diskDC != c.lastDiskDC {
			if err := c.diskPWM.SetDutyCycle(diskDC); err != nil {
				return err
			}
			c.lastDiskDC = diskDC
		}
	}

	logger.Infof("cpu_temp: %.2f, cpu_dc: %.2f, disk_temp: %.2f, disk_dc: %.2f, run: %t",
		cpuTemp, cpuDC*100, diskTemp, diskDC*100, c.enabled)

	return nil
}

func (c *Controller) getTemperatures() (cpu, disk float64) {
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
			cpu = temp / 1000.0
		}
	}

	if c.cfg.Fan.TempDisks && time.Since(c.lastTemp) > 10*time.Second {
		c.lastDiskTemp = c.getMaxDiskTemp()
		c.lastTemp = time.Now()
	}
	disk = c.lastDiskTemp

	return cpu, disk
}

func (c *Controller) getMaxDiskTemp() float64 {
	disks := disk.GetSATADisks()
	if len(disks) == 0 {
		return 0
	}

	var maxTemp float64
	for _, diskDev := range disks {
		temp, err := disk.GetTemperature(diskDev)
		if err != nil {
			continue
		}
		if temp > maxTemp {
			maxTemp = temp
		}
	}

	return maxTemp
}

func (c *Controller) calculateDutyCycle(temp float64, key byte) float64 {
	var lv0, lv1, lv2, lv3, maxTemp float64

	if key == 'c' {
		lv0, lv1, lv2, lv3 = c.cfg.Fan.LV0C, c.cfg.Fan.LV1C, c.cfg.Fan.LV2C, c.cfg.Fan.LV3C
		maxTemp = c.cfg.Fan.MaxCPUTemp
	} else {
		lv0, lv1, lv2, lv3 = c.cfg.Fan.LV0F, c.cfg.Fan.LV1F, c.cfg.Fan.LV2F, c.cfg.Fan.LV3F
		maxTemp = c.cfg.Fan.MaxDiskTemp
	}

	if c.cfg.Fan.Linear {
		return c.linearInterpolate(temp, lv0, lv1, lv2, lv3, maxTemp)
	}

	if temp < lv0 {
		return 0
	} else if temp < lv1 {
		return 0.25
	} else if temp < lv2 {
		return 0.50
	} else if temp < lv3 {
		return 0.75
	}
	return 1.0
}

func (c *Controller) linearInterpolate(temp, lv0, lv1, lv2, lv3, maxTemp float64) float64 {
	if temp < lv0 {
		return 0
	}

	levels := []float64{lv0, lv1, lv2, lv3, maxTemp}
	dutyCycles := []float64{0.01, 0.25, 0.50, 0.75, 1.0}

	for i := 0; i < len(levels)-1; i++ {
		if temp >= levels[i] && temp < levels[i+1] {
			ratio := (temp - levels[i]) / (levels[i+1] - levels[i])
			return dutyCycles[i] + ratio*(dutyCycles[i+1]-dutyCycles[i])
		}
	}

	return 1.0
}

// GetFanSpeeds returns the current CPU and disk fan duty cycles as percentages (0-100)
func (c *Controller) GetFanSpeeds() (cpuPercent, diskPercent float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastCPUDC * 100, c.lastDiskDC * 100
}

func (c *Controller) Close() error {
	if c.cpuPWM != nil {
		c.cpuPWM.SetDutyCycle(0)
		c.cpuPWM.Close()
	}
	if c.diskPWM != nil {
		c.diskPWM.SetDutyCycle(0)
		c.diskPWM.Close()
	}
	return nil
}
