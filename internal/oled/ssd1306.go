package oled

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"
	"time"

	i2c "github.com/d2r2/go-i2c"
	i2cl "github.com/d2r2/go-logger"
	"github.com/warthog618/go-gpiocdev"

	"github.com/kolobock/rockpi-quad-go/internal/logger"
)

// SSD1306 command constants
const (
	ssd1306SetContrast        = 0x81
	ssd1306DisplayAllOnResume = 0xA4
	ssd1306DisplayAllOn       = 0xA5
	ssd1306NormalDisplay      = 0xA6
	ssd1306InvertDisplay      = 0xA7
	ssd1306DisplayOff         = 0xAE
	ssd1306DisplayOn          = 0xAF
	ssd1306SetDisplayOffset   = 0xD3
	ssd1306SetComPins         = 0xDA
	ssd1306SetVcomDetect      = 0xDB
	ssd1306SetDisplayClockDiv = 0xD5
	ssd1306SetPrecharge       = 0xD9
	ssd1306SetMultiplex       = 0xA8
	ssd1306SetLowColumn       = 0x00
	ssd1306SetHighColumn      = 0x10
	ssd1306SetStartLine       = 0x40
	ssd1306MemoryMode         = 0x20
	ssd1306ColumnAddr         = 0x21
	ssd1306PageAddr           = 0x22
	ssd1306ComScanInc         = 0xC0
	ssd1306ComScanDec         = 0xC8
	ssd1306SegRemap           = 0xA0
	ssd1306ChargePump         = 0x8D
	ssd1306DeactivateScroll   = 0x2E
	ssd1306ExternalVcc        = 0x01
	ssd1306SwitchCapVcc       = 0x02

	ssd1306I2CAddr = 0x3C
)

// SSD1306 represents an SSD1306 OLED display driver
type SSD1306 struct {
	i2c    *i2c.I2C
	width  int
	height int
	buffer []byte
}

// NewSSD1306 creates a new SSD1306 driver instance
func NewSSD1306(width, height int) (*SSD1306, error) {
	if err := i2cl.ChangePackageLogLevel("i2c", i2cl.InfoLevel); err != nil {
		logger.Infof("Failed to change i2c log level: %v", err)
	}

	i2cBus, err := i2c.NewI2C(ssd1306I2CAddr, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to open I2C: %w", err)
	}

	d := &SSD1306{
		i2c:    i2cBus,
		width:  width,
		height: height,
		buffer: make([]byte, width*height/8),
	}
	logger.Infof("[SSD1306] Initialized %dx%d display, buffer size: %d bytes", width, height, len(d.buffer))

	if err := d.reset(); err != nil {
		i2cBus.Close()
		return nil, fmt.Errorf("failed to reset SSD1306: %w", err)
	}

	if err := d.init(); err != nil {
		i2cBus.Close()
		return nil, fmt.Errorf("failed to initialize SSD1306: %w", err)
	}

	return d, nil
}

// reset performs a hardware reset of the SSD1306 display using GPIO
func (d *SSD1306) reset() error {
	chip, err := gpiocdev.NewChip("gpiochip0")
	if err != nil {
		return fmt.Errorf("cannot open gpiochip0: %w", err)
	}
	defer chip.Close()

	resetPin := os.Getenv("OLED_RESET") // "D23"
	if resetPin == "" {
		return nil
	}

	resetPin = strings.ToLower(resetPin)
	resetPin = strings.TrimPrefix(resetPin, "d")

	pinNum, err := strconv.Atoi(resetPin)
	if err != nil {
		return fmt.Errorf("invalid OLED_RESET pin: %w", err)
	}

	line, err := chip.RequestLine(pinNum, gpiocdev.AsOutput(0))
	if err != nil {
		return fmt.Errorf("cannot request gpio line: %w", err)
	}
	defer line.Close()

	time.Sleep(10 * time.Millisecond)

	if err := line.SetValue(1); err != nil {
		return fmt.Errorf("cannot set gpio high: %w", err)
	}

	time.Sleep(10 * time.Millisecond)

	return nil
}

// init initializes the SSD1306 display with proper configuration
func (d *SSD1306) init() error {
	cmds := []byte{
		ssd1306DisplayOff,
		ssd1306MemoryMode, 0x00, // 0x00 from tinygo, 0x02 working but stops with some time
		ssd1306SetDisplayClockDiv, 0x80,
		ssd1306SetMultiplex, byte(d.height - 1),
		ssd1306SetDisplayOffset, 0x00,
		ssd1306SetStartLine,
		ssd1306SegRemap | 0x01,
		ssd1306ComScanDec,
	}

	switch d.height {
	case 32:
		cmds = append(cmds, ssd1306SetComPins, 0x02)
	case 64:
		cmds = append(cmds, ssd1306SetComPins, 0x12)
	}

	cmds = append(cmds,
		ssd1306SetPrecharge, 0xF1,
		ssd1306SetVcomDetect, 0x40,
		ssd1306SetContrast, 0x8F, // 0x8F from tinygo, 0xFF was
		ssd1306DisplayAllOnResume,
		ssd1306NormalDisplay,
		ssd1306DeactivateScroll,
		ssd1306ChargePump, 0x14,
	)

	for _, cmd := range cmds {
		if err := d.writeCmd(cmd); err != nil {
			return err
		}
	}

	if err := d.writeCmd(ssd1306DisplayOn); err != nil {
		return err
	}

	return d.Clear()
}

// writeCmd sends a command byte to the display
func (d *SSD1306) writeCmd(cmd byte) error {
	_, err := d.i2c.WriteBytes([]byte{0x00, cmd})
	return err
}

// Display updates the OLED display with the contents of the image
func (d *SSD1306) Display(img *image.Gray) error {
	for page := 0; page < d.height/8; page++ {
		for x := 0; x < d.width; x++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				y := page*8 + bit
				if img.GrayAt(x, y).Y > 128 {
					b |= (1 << bit)
				}
			}
			d.buffer[page*d.width+x] = b
		}
	}
	for page := 0; page < d.height/8; page++ {
		if err := d.writeCmd(0xB0 | byte(page)); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetLowColumn); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetHighColumn); err != nil {
			return err
		}

		pageData := make([]byte, d.width+1)
		pageData[0] = 0x40
		copy(pageData[1:], d.buffer[page*d.width:(page+1)*d.width])

		if _, err := d.i2c.WriteBytes(pageData); err != nil {
			return err
		}
	}

	return nil
}

// Clear clears the display (turns all pixels off)
func (d *SSD1306) Clear() error {
	for i := range d.buffer {
		d.buffer[i] = 0
	}

	zeroPage := make([]byte, d.width+1)
	zeroPage[0] = 0x40

	for page := 0; page < d.height/8; page++ {
		if err := d.writeCmd(0xB0 | byte(page)); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetLowColumn); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetHighColumn); err != nil {
			return err
		}
		if _, err := d.i2c.WriteBytes(zeroPage); err != nil {
			return err
		}
	}
	return nil
}

// SetContrast sets the display contrast (0-255)
func (d *SSD1306) SetContrast(contrast byte) error {
	if err := d.writeCmd(ssd1306SetContrast); err != nil {
		return err
	}
	return d.writeCmd(contrast)
}

// SetDisplayOn turns the display on or off
func (d *SSD1306) SetDisplayOn(on bool) error {
	if on {
		return d.writeCmd(ssd1306DisplayOn)
	}
	return d.writeCmd(ssd1306DisplayOff)
}

// Close closes the I2C connection and turns off the display
func (d *SSD1306) Close() error {
	if err := d.SetDisplayOn(false); err != nil {
		logger.Errorf("Failed to turn off display: %v", err)
	}
	return d.i2c.Close()
}
