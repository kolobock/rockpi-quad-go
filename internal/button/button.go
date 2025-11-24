package button

import (
	"context"
	"fmt"
	"log/syslog"
	"strings"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/config"
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
	cfg         *config.Config
	line        *gpiocdev.Line
	pressChan   chan EventType
	syslogger   *syslog.Writer
	twiceWindow time.Duration
	pressTime   time.Duration
	eventChan   chan gpiocdev.LineEvent
}

// New creates a new button controller using chip and line number
func New(cfg *config.Config) (*Controller, error) {
	chip := cfg.Env.ButtonChip
	line := cfg.Env.ButtonLine
	twiceWindow := cfg.Time.Twice
	pressTime := cfg.Time.Press

	ctrl := &Controller{
		cfg:         cfg,
		pressChan:   make(chan EventType, 10),
		twiceWindow: time.Duration(twiceWindow * float64(time.Second)),
		pressTime:   time.Duration(pressTime * float64(time.Second)),
	}

	if cfg.Fan.Syslog {
		syslogger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad-go")
		if err != nil {
			return nil, err
		}
		ctrl.syslogger = syslogger
	}

	if line == "" {
		if ctrl.syslogger != nil {
			ctrl.syslogger.Info("Button monitoring disabled - no pin configured")
		}
		return ctrl, nil
	}

	if chip == "" {
		chip = "gpiochip0"
	}

	var chipNum int
	if _, err := fmt.Sscanf(chip, "%d", &chipNum); err == nil {
		chip = "gpiochip" + chip
	}

	if !strings.HasPrefix(chip, "/dev/") {
		chip = "/dev/" + chip
	}

	lineNum := 0
	if _, err := fmt.Sscanf(line, "%d", &lineNum); err != nil {
		if ctrl.syslogger != nil {
			ctrl.syslogger.Warning("Invalid GPIO line number: " + line)
		}
		return ctrl, nil
	}

	ctrl.eventChan = make(chan gpiocdev.LineEvent, 10)

	eventHandler := func(evt gpiocdev.LineEvent) {
		select {
		case ctrl.eventChan <- evt:
		default:
		}
	}

	l, err := gpiocdev.RequestLine(chip, lineNum,
		gpiocdev.AsInput,
		gpiocdev.WithPullUp,
		gpiocdev.WithBothEdges,
		gpiocdev.WithEventHandler(eventHandler))
	if err != nil {
		if ctrl.syslogger != nil {
			ctrl.syslogger.Warning("Failed to request button line: " + err.Error())
		}
		return ctrl, nil
	}

	ctrl.line = l
	time.Sleep(100 * time.Millisecond)
	for len(ctrl.eventChan) > 0 {
		<-ctrl.eventChan
	}
	if ctrl.syslogger != nil {
		ctrl.syslogger.Info("Button monitoring enabled on " + chip + " line " + line)
	}
	return ctrl, nil
}

// Run starts monitoring button presses and detects click/double-click/long-press
func (c *Controller) Run(ctx context.Context) {
	if c.line == nil {
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
					if c.syslogger != nil {
						c.syslogger.Info("Button event: " + string(event))
					}
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// detectButtonEvent waits for and detects the type of button press
func (c *Controller) detectButtonEvent(ctx context.Context) EventType {
	var pressStart time.Time
	for {
		select {
		case <-ctx.Done():
			return ""
		case evt := <-c.eventChan:
			if evt.Type == gpiocdev.LineEventFallingEdge {
				pressStart = time.Now()
				goto waitForRelease
			}
		case <-time.After(200 * time.Millisecond):
			return ""
		}
	}

waitForRelease:
	for {
		select {
		case <-ctx.Done():
			return ""
		case evt := <-c.eventChan:
			if evt.Type == gpiocdev.LineEventRisingEdge {
				goto checkDoubleClick
			}
		case <-time.After(50 * time.Millisecond):
			if time.Since(pressStart) >= c.pressTime {
				for {
					select {
					case <-ctx.Done():
						return LongPress
					case evt := <-c.eventChan:
						if evt.Type == gpiocdev.LineEventRisingEdge {
							return LongPress
						}
					case <-time.After(50 * time.Millisecond):
					}
				}
			}
		}
	}

checkDoubleClick:
	deadline := time.Now().Add(c.twiceWindow)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return Click
		case evt := <-c.eventChan:
			if evt.Type == gpiocdev.LineEventFallingEdge {
				for {
					select {
					case <-ctx.Done():
						return DoubleClick
					case evt := <-c.eventChan:
						if evt.Type == gpiocdev.LineEventRisingEdge {
							c.drainEventChannel()
							return DoubleClick
						}
					case <-time.After(50 * time.Millisecond):
					}
				}
			}
		case <-time.After(deadline.Sub(time.Now())):
			return Click
		}
	}

	return Click
}

// drainEventChannel clears any pending events from the event channel
func (c *Controller) drainEventChannel() {
	for {
		select {
		case <-c.eventChan:
		default:
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
