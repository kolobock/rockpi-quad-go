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

func main() {
	cfg, err := config.Load("/etc/rockpi-quad.conf")
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	logger.SetVerbose(cfg.Fan.Syslog)

	disk.EnableSATAController(cfg.Env.SATAChip, cfg.Env.SATALine1, cfg.Env.SATALine2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	fanCtrl, err := fan.New(cfg)
	if err != nil {
		logger.Fatalf("Failed to create fan controller: %v", err)
	}
	defer fanCtrl.Close()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := fanCtrl.Run(ctx); err != nil {
			logger.Errorf("Fan controller error: %v", err)
		}
	}()

	if cfg.OLED.Enabled {
		buttonCtrl, err := button.New(cfg)
		if err != nil {
			logger.Errorf("Failed to create button controller: %v", err)
			goto oled
		}
		defer buttonCtrl.Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			buttonCtrl.Run(ctx)
		}()

	oled:
		oledCtrl, err := oled.New(cfg, fanCtrl)
		if err != nil {
			logger.Errorf("Failed to create OLED controller: %v", err)
			goto sigChannel
		}
		defer oledCtrl.Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			buttonChan := make(chan struct{}, 10)

			if buttonCtrl != nil {
				go func() {
					time.Sleep(500 * time.Millisecond)

					for event := range buttonCtrl.PressChan() {
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
							logger.Infoln("Poweroff requested via button press")
							go func() {
								time.Sleep(1 * time.Second)
								if err := exec.Command("poweroff").Run(); err != nil {
									logger.Errorf("Failed to execute poweroff: %v", err)
								}
							}()
							cancel()
						case "reboot":
							logger.Infoln("Reboot requested via button press")
							go func() {
								time.Sleep(1 * time.Second)
								if err := exec.Command("reboot").Run(); err != nil {
									logger.Errorf("Failed to execute reboot: %v", err)
								}
							}()
							cancel()
						case "none":
						default:
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
					}
				}()
			}
			if err := oledCtrl.Run(ctx, buttonChan); err != nil {
				logger.Errorf("OLED controller error: %v", err)
			}
		}()
	}

sigChannel:
	<-sigCh
	logger.Infoln("Shutting down...")
	cancel()

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
		return "none"
	}
}
