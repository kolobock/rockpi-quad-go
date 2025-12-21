package oled

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"testing"
	"time"

	"golang.org/x/image/font"

	"github.com/kolobock/rockpi-quad-go/internal/config"
)

func TestClearImage(t *testing.T) {
	ctrl := &Controller{
		img: image.NewGray(image.Rect(0, 0, 128, 32)),
	}

	for y := 0; y < 32; y++ {
		for x := 0; x < 128; x++ {
			ctrl.img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	ctrl.clearImage()

	for y := 0; y < 32; y++ {
		for x := 0; x < 128; x++ {
			if ctrl.img.GrayAt(x, y).Y != 0 {
				t.Errorf("pixel at (%d, %d) = %v, want 0", x, y, ctrl.img.GrayAt(x, y).Y)
			}
		}
	}
}

func TestRotateImage180(t *testing.T) {
	ctrl := &Controller{}
	src := image.NewGray(image.Rect(0, 0, 4, 4))

	src.SetGray(0, 0, color.Gray{Y: 255})
	src.SetGray(3, 3, color.Gray{Y: 200})

	dst := ctrl.rotateImage180(src)

	if dst.GrayAt(3, 3).Y != 255 {
		t.Errorf("rotated pixel at (3,3) = %v, want 255", dst.GrayAt(3, 3).Y)
	}
	if dst.GrayAt(0, 0).Y != 200 {
		t.Errorf("rotated pixel at (0,0) = %v, want 200", dst.GrayAt(0, 0).Y)
	}
}

func TestConstants(t *testing.T) {
	if displayWidth != 128 {
		t.Errorf("displayWidth = %v, want 128", displayWidth)
	}
	if displayHeight != 32 {
		t.Errorf("displayHeight = %v, want 32", displayHeight)
	}
}
func TestControllerContextCancellation(t *testing.T) {
	// This test verifies that the controller properly handles context cancellation
	// without attempting to use a closed device - regression test for the
	// "file already closed" error when showGoodbye() runs after Close()

	// Create a mock device that tracks if operations happen after Close()
	mockDev := &mockSSD1306{
		closed:       false,
		closeCount:   0,
		displayCalls: make([]bool, 0),
	}

	ctrl := &Controller{
		cfg: &config.Config{
			OLED: config.OLEDConfig{
				Enabled:    true,
				Rotate:     false,
				Fahrenheit: false,
			},
			Slider: config.SliderConfig{
				Auto: false,
				Time: 1,
			},
			Disk: config.DiskConfig{
				SpaceUsageMountPoints: []string{},
				IOUsageMountPoints:    []string{},
				DisksTemperature:      false,
			},
			Network: config.NetworkConfig{
				Interfaces: []string{},
				SkipPage:   true,
			},
		},
		dev:           mockDev,
		img:           image.NewGray(image.Rect(0, 0, displayWidth, displayHeight)),
		netStats:      make(map[string]netIOStats),
		diskStats:     make(map[string]diskIOStats),
		fonts:         make(map[int]font.Face),
		timerDuration: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	buttonChan := make(chan struct{})

	// Start Run in a goroutine (simulating the actual usage)
	runComplete := make(chan struct{})
	go func() {
		defer close(runComplete)
		_ = ctrl.Run(ctx, buttonChan)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context and wait for Run to complete
	cancel()
	<-runComplete

	// Now close the controller (this simulates what happens when defer executes)
	if err := ctrl.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify that no display calls happened after Close()
	if mockDev.displayAfterClose {
		t.Error("Display() was called after Close() - this indicates a race condition")
	}

	// Verify Close was called exactly once
	if mockDev.closeCount != 1 {
		t.Errorf("Close() called %d times, want 1", mockDev.closeCount)
	}
}

type mockSSD1306 struct {
	closed            bool
	closeCount        int
	displayCalls      []bool
	displayAfterClose bool
}

func (m *mockSSD1306) Display(img *image.Gray) error {
	m.displayCalls = append(m.displayCalls, m.closed)
	if m.closed {
		m.displayAfterClose = true
		return fmt.Errorf("write /dev/i2c-1: file already closed")
	}
	return nil
}

func (m *mockSSD1306) Clear() error {
	if m.closed {
		return fmt.Errorf("write /dev/i2c-1: file already closed")
	}
	return nil
}

func (m *mockSSD1306) Close() error {
	m.closeCount++
	m.closed = true
	return nil
}
