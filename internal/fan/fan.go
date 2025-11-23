package fan

import (
	"context"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/pkg/pwm"
)

const (
	MinDutyCycle = 0.07 // 7% minimum for fan to spin
)

type Controller struct {
	cfg       *config.Config
	cpuPWM    *pwm.PWM
	diskPWM   *pwm.PWM
	syslogger *syslog.Writer

	lastCPUDC  float64
	lastDiskDC float64
	lastTemp   time.Time
}

func New(cfg *config.Config) (*Controller, error) {
	ctrl := &Controller{
		cfg:      cfg,
		lastTemp: time.Now().Add(-time.Hour), // Force first read
	}

	// Initialize CPU fan PWM
	cpuPWM, err := pwm.New(cfg.Fan.CPUPWMChip, cfg.Fan.CPUPWMChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to init CPU PWM: %w", err)
	}
	ctrl.cpuPWM = cpuPWM

	// Set polarity
	if cfg.Fan.Polarity == "inversed" {
		cpuPWM.SetInversed(true)
	}

	// Initialize disk fan PWM if different channel
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

	// Initialize syslog if enabled
	if cfg.Fan.Syslog {
		logger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad")
		if err == nil {
			ctrl.syslogger = logger
		}
	}

	return ctrl, nil
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
				log.Printf("Fan update error: %v", err)
			}
		}
	}
}

func (c *Controller) update() error {
	cpuTemp, diskTemp := c.getTemperatures()

	cpuDC := c.calculateDutyCycle(cpuTemp, 'c')
	diskDC := c.calculateDutyCycle(diskTemp, 'f')

	// Apply minimum duty cycle threshold
	if cpuDC > 0 && cpuDC < MinDutyCycle {
		cpuDC = MinDutyCycle
	}
	if diskDC > 0 && diskDC < MinDutyCycle {
		diskDC = MinDutyCycle
	}

	// Update PWM if changed
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

	// Log to syslog
	if c.syslogger != nil {
		c.syslogger.Info(fmt.Sprintf("cpu_temp: %.2f, cpu_dc: %.2f, disk_temp: %.2f, disk_dc: %.2f",
			cpuTemp, cpuDC*100, diskTemp, diskDC*100))
	}

	return nil
}

func (c *Controller) getTemperatures() (cpu, disk float64) {
	// Read CPU temperature
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
			cpu = temp / 1000.0
		}
	}

	// Read disk temperature if enabled
	if c.cfg.Fan.TempDisks && time.Since(c.lastTemp) > 10*time.Second {
		disk = c.getMaxDiskTemp()
		c.lastTemp = time.Now()
	}

	return cpu, disk
}

func (c *Controller) getMaxDiskTemp() float64 {
	// TODO: Implement smartctl disk temperature reading
	// For now, read from a simple cache file if it exists
	if data, err := os.ReadFile("/tmp/rockpi-disk-temp"); err == nil {
		if temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
			return temp
		}
	}
	return 0
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

	// Non-linear (step-based)
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
			// Linear interpolation between two points
			ratio := (temp - levels[i]) / (levels[i+1] - levels[i])
			return dutyCycles[i] + ratio*(dutyCycles[i+1]-dutyCycles[i])
		}
	}

	return 1.0
}

func (c *Controller) Close() error {
	// Set fans to 0 before closing
	if c.cpuPWM != nil {
		c.cpuPWM.SetDutyCycle(0)
		c.cpuPWM.Close()
	}
	if c.diskPWM != nil {
		c.diskPWM.SetDutyCycle(0)
		c.diskPWM.Close()
	}
	if c.syslogger != nil {
		c.syslogger.Close()
	}
	return nil
}
