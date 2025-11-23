package oled

import (
	"context"

	"github.com/kolobock/rockpi-quad-go/internal/config"
)

type Controller struct {
	cfg *config.Config
}

func New(cfg *config.Config) (*Controller, error) {
	// TODO: Implement OLED display using periph.io
	return &Controller{cfg: cfg}, nil
}

func (c *Controller) Run(ctx context.Context) error {
	// TODO: Implement OLED page cycling
	<-ctx.Done()
	return nil
}

func (c *Controller) Close() error {
	return nil
}
