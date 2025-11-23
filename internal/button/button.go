package button

import (
	"context"
	"log/syslog"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

// EventType represents the type of button event
type EventType string

const (
	Click       EventType = "click"
	DoubleClick EventType = "twice"
	LongPress   EventType = "press"
)

// Controller handles button press monitoring
type Controller struct {
	pin         gpio.PinIn
	pressChan   chan EventType
	syslogger   *syslog.Writer
	twiceWindow time.Duration // time window for double-click detection
	pressTime   time.Duration // time threshold for long-press detection
}

// New creates a new button controller using chip and line number
func New(chip, line string, twiceWindow, pressTime float64) (*Controller, error) {
	syslogger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad-go")
	if err != nil {
		return nil, err
	}

	if line == "" {
		syslogger.Info("Button monitoring disabled - no pin configured")
		return &Controller{
			pressChan:   make(chan EventType, 10),
			syslogger:   syslogger,
			twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
			pressTime:   time.Duration(pressTime * float64(time.Second)),
		}, nil
	}

	// For Rock Pi 4, gpiochip0 line 17 corresponds to GPIO17
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
			pressChan:   make(chan EventType, 10),
			syslogger:   syslogger,
			twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
			pressTime:   time.Duration(pressTime * float64(time.Second)),
		}, nil
	}

	if err := pin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		syslogger.Warning("Failed to setup button pin: " + err.Error())
		return &Controller{
			pressChan:   make(chan EventType, 10),
			syslogger:   syslogger,
			twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
			pressTime:   time.Duration(pressTime * float64(time.Second)),
		}, nil
	}

	syslogger.Info("Button monitoring enabled on " + pin.Name())
	return &Controller{
		pin:         pin,
		pressChan:   make(chan EventType, 10),
		syslogger:   syslogger,
		twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
		pressTime:   time.Duration(pressTime * float64(time.Second)),
	}, nil
}

// Run starts monitoring button presses and detects click/double-click/long-press
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
			event := c.detectButtonEvent(ctx)
			if event != "" {
				select {
				case c.pressChan <- event:
					c.syslogger.Info("Button event: " + string(event))
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// detectButtonEvent waits for and detects the type of button press
func (c *Controller) detectButtonEvent(ctx context.Context) EventType {
	// Wait for button press (falling edge)
	if !c.pin.WaitForEdge(200 * time.Millisecond) {
		return ""
	}

	// Debounce
	time.Sleep(50 * time.Millisecond)
	if c.pin.Read() != gpio.Low {
		return "" // False trigger
	}

	// Record press start time
	pressStart := time.Now()

	// Wait for button release or long-press timeout
	for c.pin.Read() == gpio.Low {
		if time.Since(pressStart) >= c.pressTime {
			// Long press detected
			// Wait for release
			for c.pin.Read() == gpio.Low {
				time.Sleep(50 * time.Millisecond)
			}
			return LongPress
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Button was released - now check for double-click
	// Wait for potential second click within the double-click window
	deadline := time.Now().Add(c.twiceWindow)
	for time.Now().Before(deadline) {
		if c.pin.WaitForEdge(deadline.Sub(time.Now())) {
			// Debounce second click
			time.Sleep(50 * time.Millisecond)
			if c.pin.Read() == gpio.Low {
				// Second click detected
				// Wait for release
				for c.pin.Read() == gpio.Low {
					time.Sleep(50 * time.Millisecond)
				}
				return DoubleClick
			}
		}
	}

	// No second click - it's a single click
	return Click
}

// PressChan returns the channel that receives button press events
func (c *Controller) PressChan() <-chan EventType {
	return c.pressChan
}

// Close cleans up resources
func (c *Controller) Close() error {
	if c.syslogger != nil {
		return c.syslogger.Close()
	}
	return nil
}
