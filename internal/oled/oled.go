package oled

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log/syslog"
	"os"
	"sync"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/kolobock/rockpi-quad-go/internal/config"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	displayWidth  = 128
	displayHeight = 32
	sliderTime    = 10 * time.Second // default page rotation time
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
	syslogger    *syslog.Writer
	fonts        map[int]font.Face
	fanCtrl      FanController
	tempDiskDevs []string // Cached list of disks for temperature monitoring
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
	// Create SSD1306 display driver
	display, err := NewSSD1306(displayWidth, displayHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSD1306 display: %w", err)
	}

	// Load TTF fonts with different sizes
	fonts := make(map[int]font.Face)
	for _, size := range []int{10, 11, 12, 14} {
		fontFace, err := loadFont("fonts/DejaVuSansMono-Bold.ttf", float64(size))
		if err != nil {
			return nil, fmt.Errorf("failed to load font size %d: %w", size, err)
		}
		fonts[size] = fontFace
	}

	c := &Controller{
		cfg:       cfg,
		dev:       display,
		img:       image.NewGray(image.Rect(0, 0, displayWidth, displayHeight)),
		netStats:  make(map[string]netIOStats),
		diskStats: make(map[string]diskIOStats),
		fonts:     fonts,
		fanCtrl:   fanCtrl,
	}

	// Initialize syslog
	logger, err := syslog.New(syslog.LOG_INFO, "rockpi-quad-go")
	if err == nil {
		c.syslogger = logger
	}

	// Initialize network and disk stats
	c.updateNetworkStats()
	c.updateDiskStats()
	c.initTempDisks()

	// Show welcome message
	c.showWelcome()

	return c, nil
}

func (c *Controller) Run(ctx context.Context, buttonChan <-chan struct{}) error {
	c.showWelcome()

	// Generate all pages
	c.pages = c.generatePages()
	if len(c.pages) == 0 {
		if c.syslogger != nil {
			c.syslogger.Info("No OLED pages configured, display disabled")
		}
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(sliderTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.showGoodbye()
			return nil
		case <-ticker.C:
			c.nextPage()
		case <-buttonChan:
			// Button pressed - advance to next page
			c.nextPage()
		}
	}
}

func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear display
	c.clearImage()
	c.displayToDevice()

	if c.syslogger != nil {
		c.syslogger.Close()
	}
	return c.dev.Close()
}

func (c *Controller) clearImage() {
	for y := 0; y < displayHeight; y++ {
		for x := 0; x < displayWidth; x++ {
			c.img.SetGray(x, y, color.Gray{Y: 0})
		}
	}
}

func (c *Controller) drawText(x, y int, text string, fontSize int) {
	// Default to size 11 if not specified or invalid
	fontFace, ok := c.fonts[fontSize]
	if !ok {
		fontFace = c.fonts[11]
	}

	// Python PIL draws from top-left corner at (x, y)
	// Go font.Drawer uses baseline, so we need to add the ascent
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
	time.Sleep(1 * time.Second)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pages) == 0 {
		return
	}

	c.pageIndex = (c.pageIndex + 1) % len(c.pages)
	page := c.pages[c.pageIndex]

	c.clearImage()
	items := page.GetPageText()
	for _, item := range items {
		c.drawText(item.X, item.Y, item.Text, item.FontSize)
	}
	c.display()
}
