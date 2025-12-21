package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kolobock/rockpi-quad-go/internal/button"
	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/disk"
	"github.com/kolobock/rockpi-quad-go/internal/fan"
	"github.com/kolobock/rockpi-quad-go/internal/logger"
	"github.com/kolobock/rockpi-quad-go/internal/oled"
)

const (
	actionNone = "none"
)

func handleButtonEvents(ctx context.Context, cfg *config.Config, buttonCtrl *button.Controller,
	fanCtrl *fan.Controller, oledCtrl *oled.Controller, buttonChan chan struct{}, cancel context.CancelFunc) {
	time.Sleep(500 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-buttonCtrl.PressChan():
			if !ok {
				// Channel closed, exit
				return
			}
			action := getButtonAction(cfg, event)
			logger.Infof("Button event: %s (action: %s)", event, action)
			oledCtrl.NotifyBtnPress()

			switch action {
			case "slider":
				select {
				case buttonChan <- struct{}{}:
				default:
				}
			case "switch":
				fanCtrl.ToggleFan()
			case "poweroff":
				executePoweroff(cancel)
			case "reboot":
				executeReboot(cancel)
			case actionNone:
			default:
				executeCustomCommand(action)
			}
		}
	}
}

func executePoweroff(cancel context.CancelFunc) {
	logger.Infoln("Poweroff requested via button press")
	go func() {
		time.Sleep(1 * time.Second)
		if err := exec.Command("poweroff").Run(); err != nil {
			logger.Errorf("Failed to execute poweroff: %v", err)
		}
	}()
	cancel()
}

func executeReboot(cancel context.CancelFunc) {
	logger.Infoln("Reboot requested via button press")
	go func() {
		time.Sleep(1 * time.Second)
		if err := exec.Command("reboot").Run(); err != nil {
			logger.Errorf("Failed to execute reboot: %v", err)
		}
	}()
	cancel()
}

func executeCustomCommand(action string) {
	logger.Infof("Executing custom command: %s", action)
	go func() {
		cmd := exec.Command("sh", "-c", action)
		if err := cmd.Run(); err != nil {
			logger.Errorf("Failed to execute command '%s': %v", action, err)
		} else {
			logger.Infof("Command '%s' executed successfully", action)
		}
	}()
}

func main() {
	cfg := loadConfigAndSetup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	fanCtrl := startFanController(ctx, &wg, cfg)
	defer fanCtrl.Close()

	if cfg.OLED.Enabled {
		startOLEDAndButton(ctx, &wg, cfg, fanCtrl, cancel)
	}

	<-sigCh
	logger.Infoln("Shutting down...")
	cancel()

	waitForShutdown(&wg)
}

func loadConfigAndSetup() *config.Config {
	cfg, err := config.Load("/etc/rockpi-quad.conf")
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	logger.SetVerbose(cfg.Fan.Syslog)
	disk.EnableSATAController(cfg.Env.SATAChip, cfg.Env.SATALine1, cfg.Env.SATALine2)

	return cfg
}

func startFanController(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) *fan.Controller {
	fanCtrl, err := fan.New(cfg)
	if err != nil {
		logger.Fatalf("Failed to create fan controller: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := fanCtrl.Run(ctx); err != nil {
			logger.Errorf("Fan controller error: %v", err)
		}
	}()

	return fanCtrl
}

func startOLEDAndButton(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, fanCtrl *fan.Controller, cancel context.CancelFunc) {
	buttonCtrl, err := button.New(cfg)
	if err != nil {
		logger.Errorf("Failed to create button controller: %v", err)
		goto oled
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer buttonCtrl.Close()
		buttonCtrl.Run(ctx)
	}()

oled:
	oledCtrl, err := oled.New(cfg, fanCtrl)
	if err != nil {
		logger.Errorf("Failed to create OLED controller: %v", err)
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer oledCtrl.Close()
		buttonChan := make(chan struct{}, 10)

		if buttonCtrl != nil {
			go handleButtonEvents(ctx, cfg, buttonCtrl, fanCtrl, oledCtrl, buttonChan, cancel)
		}
		if err := oledCtrl.Run(ctx, buttonChan); err != nil {
			logger.Errorf("OLED controller error: %v", err)
		}
	}()
}

func waitForShutdown(wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Infoln("Shutdown complete")
	case <-time.After(5 * time.Second):
		logger.Infoln("Shutdown timeout")
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
		return actionNone
	}
}
