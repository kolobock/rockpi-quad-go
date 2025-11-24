package button

import (
	"context"
	"fmt"
	"log/syslog"
	"strings"
	"time"

	"github.com/warthog618/go-gpiocdev"
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
	line        *gpiocdev.Line
	pressChan   chan EventType
	syslogger   *syslog.Writer
	twiceWindow time.Duration // time window for double-click detection
	pressTime   time.Duration // time threshold for long-press detection
	eventChan   chan gpiocdev.LineEvent
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

	// Default to gpiochip0 if not specified
	if chip == "" {
		chip = "gpiochip0"
	}

	// If chip is just a number, prepend "gpiochip"
	var chipNum int
	if _, err := fmt.Sscanf(chip, "%d", &chipNum); err == nil {
		chip = "gpiochip" + chip
	}

	// Ensure chip path starts with /dev/
	if !strings.HasPrefix(chip, "/dev/") {
		chip = "/dev/" + chip
	}

	// Convert line string to int
	lineNum := 0
	if _, err := fmt.Sscanf(line, "%d", &lineNum); err != nil {
		syslogger.Warning("Invalid GPIO line number: " + line)
		return &Controller{
			pressChan:   make(chan EventType, 10),
			syslogger:   syslogger,
			twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
			pressTime:   time.Duration(pressTime * float64(time.Second)),
		}, nil
	}

	ctrl := &Controller{
		pressChan:   make(chan EventType, 10),
		syslogger:   syslogger,
		twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
		pressTime:   time.Duration(pressTime * float64(time.Second)),
		eventChan:   make(chan gpiocdev.LineEvent, 10),
	}

	// Create event handler that forwards events to our channel
	eventHandler := func(evt gpiocdev.LineEvent) {
		select {
		case ctrl.eventChan <- evt:
		default:
			// Channel full, skip
		}
	}

	// Open GPIO chip and request line as input with pull-up and both edge detection
	l, err := gpiocdev.RequestLine(chip, lineNum,
		gpiocdev.AsInput,
		gpiocdev.WithPullUp,
		gpiocdev.WithBothEdges,
		gpiocdev.WithEventHandler(eventHandler))
	if err != nil {
		syslogger.Warning("Failed to request button line: " + err.Error())
		return &Controller{
			pressChan:   make(chan EventType, 10),
			syslogger:   syslogger,
			twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
			pressTime:   time.Duration(pressTime * float64(time.Second)),
		}, nil
	}

	ctrl.line = l
	// Drain any initial events from startup noise
	time.Sleep(100 * time.Millisecond)
	for len(ctrl.eventChan) > 0 {
		<-ctrl.eventChan
	}
	syslogger.Info("Button monitoring enabled on " + chip + " line " + line)
	return ctrl, nil
}

// Run starts monitoring button presses and detects click/double-click/long-press
func (c *Controller) Run(ctx context.Context) {
	if c.line == nil {
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
	var pressStart time.Time
	for {
		select {
		case <-ctx.Done():
			return ""
		case evt := <-c.eventChan:
			// Check if it's a falling edge (button pressed)
			if evt.Type == gpiocdev.LineEventFallingEdge {
				pressStart = time.Now()
				goto waitForRelease
			}
			// Ignore rising edges while waiting for press
		case <-time.After(200 * time.Millisecond):
			return ""
		}
	}

waitForRelease:
	// Wait for button release (rising edge) or long-press timeout
	for {
		select {
		case <-ctx.Done():
			return ""
		case evt := <-c.eventChan:
			if evt.Type == gpiocdev.LineEventRisingEdge {
				// Button released - check for double-click
				goto checkDoubleClick
			}
		case <-time.After(50 * time.Millisecond):
			if time.Since(pressStart) >= c.pressTime {
				// Long press detected - wait for release
				for {
					select {
					case <-ctx.Done():
						return LongPress
					case evt := <-c.eventChan:
						if evt.Type == gpiocdev.LineEventRisingEdge {
							return LongPress
						}
					case <-time.After(50 * time.Millisecond):
						// Continue waiting
					}
				}
			}
		}
	}

checkDoubleClick:
	// Wait for potential second click within the double-click window
	deadline := time.Now().Add(c.twiceWindow)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return Click
		case evt := <-c.eventChan:
			if evt.Type == gpiocdev.LineEventFallingEdge {
				// Second click detected - wait for release and drain channel
				for {
					select {
					case <-ctx.Done():
						return DoubleClick
					case evt := <-c.eventChan:
						if evt.Type == gpiocdev.LineEventRisingEdge {
							// Drain any remaining events in the channel
							c.drainEventChannel()
							return DoubleClick
						}
					case <-time.After(50 * time.Millisecond):
						// Continue waiting
					}
				}
			}
		case <-time.After(deadline.Sub(time.Now())):
			// Timeout - single click
			return Click
		}
	}

	// No second click - it's a single click
	return Click
}

// drainEventChannel clears any pending events from the event channel
func (c *Controller) drainEventChannel() {
	for {
		select {
		case <-c.eventChan:
			// Drain the event
		default:
			// Channel is empty
			return
		}
	}
}

// PressChan returns the channel that receives button press events
func (c *Controller) PressChan() <-chan EventType {
	return c.pressChan
}

// Close cleans up resources
func (c *Controller) Close() error {
	if c.line != nil {
		c.line.Close()
	}
	if c.syslogger != nil {
		return c.syslogger.Close()
	}
	return nil
}
