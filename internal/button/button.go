package button

import (
	"context"
	"log/syslog"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

// Controller handles button press monitoring
type Controller struct {
	pin       gpio.PinIn
	pressChan chan struct{}
	syslogger *syslog.Writer
}

// New creates a new button controller using chip and line number
func New(chip, line string) (*Controller, error) {
	syslogger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad-go")
	if err != nil {
		return nil, err
	}

	if line == "" {
		syslogger.Info("Button monitoring disabled - no pin configured")
		return &Controller{
			pressChan: make(chan struct{}, 10),
			syslogger: syslogger,
		}, nil
	}

	// For Rock Pi 4, gpiochip0 line 17 corresponds to GPIO0_C1 (pin 11)
	// Try common GPIO naming patterns
	pinNames := []string{
		"GPIO0_C1", // Rock Pi 4 naming
		"17",       // Line number
		"GPIO" + line,
	}

	var pin gpio.PinIn
	for _, name := range pinNames {
		pin = gpioreg.ByName(name)
		if pin != nil {
			break
		}
	}

	if pin == nil {
		syslogger.Warning("Button pin not found for chip " + chip + " line " + line)
		return &Controller{
			pressChan: make(chan struct{}, 10),
			syslogger: syslogger,
		}, nil
	}

	if err := pin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		syslogger.Warning("Failed to setup button pin: " + err.Error())
		return &Controller{
			pressChan: make(chan struct{}, 10),
			syslogger: syslogger,
		}, nil
	}

	syslogger.Info("Button monitoring enabled on " + pin.Name())
	return &Controller{
		pin:       pin,
		pressChan: make(chan struct{}, 10),
		syslogger: syslogger,
	}, nil
}

// Run starts monitoring button presses
func (c *Controller) Run(ctx context.Context) {
	if c.pin == nil {
		// No button configured, just wait for context cancellation
		<-ctx.Done()
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if c.pin.WaitForEdge(200 * time.Millisecond) {
				// Debounce - wait a bit and check if still low
				time.Sleep(50 * time.Millisecond)
				if c.pin.Read() == gpio.Low {
					// Button is pressed (active low with pull-up)
					select {
					case c.pressChan <- struct{}{}:
						c.syslogger.Info("Button pressed")
					default:
						// Channel full, skip
					}
					// Wait for button release
					for c.pin.Read() == gpio.Low {
						time.Sleep(50 * time.Millisecond)
					}
				}
			}
		}
	}
}

// PressChan returns the channel that receives button press events
func (c *Controller) PressChan() <-chan struct{} {
	return c.pressChan
}

// Close cleans up resources
func (c *Controller) Close() error {
	if c.syslogger != nil {
		return c.syslogger.Close()
	}
	return nil
}
