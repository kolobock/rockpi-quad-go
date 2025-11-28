package oled

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"sync"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/kolobock/rockpi-quad-go/internal/config"
	"github.com/kolobock/rockpi-quad-go/internal/logger"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	displayWidth  = 128
	displayHeight = 32
)

// FanController interface for getting fan speeds
type FanController interface {
	GetFanSpeeds() (cpuPercent, diskPercent float64)
}

type Controller struct {
	cfg          *config.Config
	dev          *SSD1306
	img          *image.Gray
	mu           sync.Mutex
	pageIndex    int
	pages        []Page
	lastIOTime   time.Time
	lastNetTime  time.Time
	netStats     map[string]netIOStats
	diskStats    map[string]diskIOStats
	fonts        map[int]font.Face
	fanCtrl      FanController
	tempDiskDevs []string

	timer         *time.Ticker
	timerDuration time.Duration
}

type netIOStats struct {
	rxBytes   uint64
	txBytes   uint64
	timestamp time.Time
}

type diskIOStats struct {
	readBytes  uint64
	writeBytes uint64
	timestamp  time.Time
}

func loadFont(path string, size float64) (font.Face, error) {
	fontBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}

	return truetype.NewFace(f, &truetype.Options{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	}), nil
}

func New(cfg *config.Config, fanCtrl FanController) (*Controller, error) {
	display, err := NewSSD1306(displayWidth, displayHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSD1306 display: %w", err)
	}

	fonts := make(map[int]font.Face)
	for _, size := range []int{10, 11, 12, 14} {
		fontFace, err := loadFont("fonts/DejaVuSansMono-Bold.ttf", float64(size))
		if err != nil {
			return nil, fmt.Errorf("failed to load font size %d: %w", size, err)
		}
		fonts[size] = fontFace
	}

	c := &Controller{
		cfg:           cfg,
		dev:           display,
		img:           image.NewGray(image.Rect(0, 0, displayWidth, displayHeight)),
		netStats:      make(map[string]netIOStats),
		diskStats:     make(map[string]diskIOStats),
		fonts:         fonts,
		fanCtrl:       fanCtrl,
		timerDuration: time.Duration(cfg.Slider.Time) * time.Second,
	}

	c.updateNetworkStats()
	c.updateDiskStats()
	c.initTempDisks()
	c.showWelcome()

	return c, nil
}

func (c *Controller) Run(ctx context.Context, buttonChan <-chan struct{}) error {
	c.pages = c.generatePages()
	if len(c.pages) == 0 {
		logger.Infoln("No OLED pages configured, display disabled")
		<-ctx.Done()
		return nil
	}

	c.nextPage()

	ticker := time.NewTicker(c.timerDuration)
	defer ticker.Stop()

	c.timer = ticker

	for {
		select {
		case <-ctx.Done():
			c.showGoodbye()
			return nil
		case <-ticker.C:
			if c.cfg.Slider.Auto {
				c.nextPage()
			}
		case <-buttonChan:
			c.nextPage()
		}
	}
}

func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearImage()
	c.displayToDevice()

	return c.dev.Close()
}

func (c *Controller) NotifyBtnPress() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.timer.Reset(c.timerDuration)
}

func (c *Controller) clearImage() {
	for y := 0; y < displayHeight; y++ {
		for x := 0; x < displayWidth; x++ {
			c.img.SetGray(x, y, color.Gray{Y: 0})
		}
	}
}

func (c *Controller) drawText(x, y int, text string, fontSize int) {
	fontFace, ok := c.fonts[fontSize]
	if !ok {
		fontFace = c.fonts[11]
	}

	metrics := fontFace.Metrics()
	ascent := metrics.Ascent.Ceil()

	point := fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y) + fixed.I(ascent),
	}

	d := &font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(color.White),
		Face: fontFace,
		Dot:  point,
	}
	d.DrawString(text)
}

func (c *Controller) display() error {
	if c.cfg.OLED.Rotate {
		rotated := c.rotateImage180(c.img)
		return c.dev.Display(rotated)
	}
	return c.displayToDevice()
}

func (c *Controller) displayToDevice() error {
	return c.dev.Display(c.img)
}

func (c *Controller) rotateImage180(src *image.Gray) *image.Gray {
	bounds := src.Bounds()
	dst := image.NewGray(bounds)
	w, h := bounds.Dx(), bounds.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(w-1-x, h-1-y, src.At(x, y))
		}
	}
	return dst
}

func (c *Controller) showWelcome() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearImage()
	c.drawText(0, 0, "ROCKPi QUAD HAT", 14)
	c.drawText(32, 16, "Loading...", 12)
	c.display()
	time.Sleep(2 * time.Second)
}

func (c *Controller) showGoodbye() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearImage()
	c.drawText(32, 8, "Good Bye ~", 14)
	c.display()
	time.Sleep(2 * time.Second)
	c.clearImage()
	c.display()
}

func (c *Controller) nextPage() {
	if len(c.pages) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.timer != nil {
		c.pageIndex = (c.pageIndex + 1) % len(c.pages)
	}
	page := c.pages[c.pageIndex]

	c.clearImage()
	items := page.GetPageText()
	for _, item := range items {
		c.drawText(item.X, item.Y, item.Text, item.FontSize)
	}
	c.display()
}
