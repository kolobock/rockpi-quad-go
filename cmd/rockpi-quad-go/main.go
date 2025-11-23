package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/button"
	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/fan"
	"github.com/kolobock/rockpi-quad-go/internal/oled"
)

func main() {
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
	buttonCtrl, err := button.New(cfg.Env.ButtonLine)
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
				buttonChan := make(<-chan struct{})
				if buttonCtrl != nil {
					buttonChan = buttonCtrl.PressChan()
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
