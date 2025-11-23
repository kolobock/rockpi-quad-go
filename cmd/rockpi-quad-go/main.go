package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/button"
	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/fan"
	"github.com/kolobock/rockpi-quad-go/internal/oled"
	"periph.io/x/host/v3"
)

func main() {
	// Initialize periph.io drivers (GPIO, I2C, etc.)
	if _, err := host.Init(); err != nil {
		log.Fatalf("Failed to initialize periph.io: %v", err)
	}

	// Load configuration
	cfg, err := config.Load("/etc/rockpi-quad.conf")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start fan controller
	fanCtrl, err := fan.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create fan controller: %v", err)
	}
	defer fanCtrl.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := fanCtrl.Run(ctx); err != nil {
			log.Printf("Fan controller error: %v", err)
		}
	}()

	// Start button controller
	buttonCtrl, err := button.New(cfg.Env.ButtonChip, cfg.Env.ButtonLine, cfg.Time.Twice, cfg.Time.Press)
	if err != nil {
		log.Printf("Failed to create button controller: %v", err)
	}
	if buttonCtrl != nil {
		defer buttonCtrl.Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			buttonCtrl.Run(ctx)
		}()
	}

	// Start OLED display if enabled
	if cfg.OLED.Enabled {
		oledCtrl, err := oled.New(cfg)
		if err != nil {
			log.Printf("Failed to create OLED controller: %v", err)
		} else {
			defer oledCtrl.Close()
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Create button channel - map all events to page advance for now
				buttonChan := make(chan struct{}, 10)
				if buttonCtrl != nil {
					go func() {
						for event := range buttonCtrl.PressChan() {
							action := getButtonAction(cfg, event)
							log.Printf("Button event: %s (action: %s)", event, action)

							// Execute the configured action
							switch action {
							case "slider":
								// Advance OLED page
								select {
								case buttonChan <- struct{}{}:
								default:
								}
							case "switch":
								// Toggle fan on/off
								fanCtrl.ToggleFan()
							case "poweroff":
								log.Println("Poweroff requested via button press")
								go func() {
									time.Sleep(1 * time.Second) // Give time for log to be written
									if err := exec.Command("poweroff").Run(); err != nil {
										log.Printf("Failed to execute poweroff: %v", err)
									}
								}()
								cancel() // Trigger shutdown
							case "reboot":
								log.Println("Reboot requested via button press")
								go func() {
									time.Sleep(1 * time.Second) // Give time for log to be written
									if err := exec.Command("reboot").Run(); err != nil {
										log.Printf("Failed to execute reboot: %v", err)
									}
								}()
								cancel() // Trigger shutdown (system will handle reboot)
							case "none":
								// Do nothing
							default:
								log.Printf("Unknown button action: %s", action)
							}
						}
					}()
				}
				if err := oledCtrl.Run(ctx, buttonChan); err != nil {
					log.Printf("OLED controller error: %v", err)
				}
			}()
		}
	}

	// Wait for signal
	<-sigCh
	log.Println("Shutting down...")
	cancel()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Shutdown complete")
	case <-time.After(5 * time.Second):
		log.Println("Shutdown timeout")
	}
}

func getButtonAction(cfg *config.Config, event button.EventType) string {
	switch event {
	case button.Click:
		return cfg.Key.Click
	case button.DoubleClick:
		return cfg.Key.Twice
	case button.LongPress:
		return cfg.Key.Press
	default:
		return "none"
	}
}
