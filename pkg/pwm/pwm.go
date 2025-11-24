package pwm

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PWM struct {
	chip     string
	channel  int
	basePath string
	period   int64
	inversed bool
}

const defaultPeriod = 40000

func New(chip string, channel int) (*PWM, error) {
	p := &PWM{
		chip:     chip,
		channel:  channel,
		basePath: fmt.Sprintf("/sys/class/pwm/%s/pwm%d", chip, channel),
		period:   defaultPeriod,
	}

	if _, err := os.Stat(p.basePath); os.IsNotExist(err) {
		exportPath := filepath.Join("/sys/class/pwm", chip, "export")
		if err := os.WriteFile(exportPath, []byte(strconv.Itoa(channel)), 0644); err != nil {
			if !strings.Contains(err.Error(), "device or resource busy") {
				return nil, fmt.Errorf("failed to export PWM: %w", err)
			}
		}
	}

	if err := p.writeSysfs("period", strconv.FormatInt(p.period, 10)); err != nil {
		return nil, err
	}

	if err := p.writeSysfs("enable", "1"); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *PWM) SetInversed(inversed bool) {
	p.inversed = inversed
	polarity := "normal"
	if inversed {
		polarity = "inversed"
	}
	p.writeSysfs("polarity", polarity)
}

func (p *PWM) SetDutyCycle(dutyCycle float64) error {
	if p.inversed {
		dutyCycle = 1.0 - dutyCycle
	}

	duty := int64(float64(p.period) * dutyCycle)
	return p.writeSysfs("duty_cycle", strconv.FormatInt(duty, 10))
}

func (p *PWM) Close() error {
	p.SetDutyCycle(0)
	return nil
}

func (p *PWM) writeSysfs(filename, value string) error {
	path := filepath.Join(p.basePath, filename)
	return os.WriteFile(path, []byte(value), 0644)
}
