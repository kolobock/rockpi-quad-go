package oled

import (
	"fmt"
	"image"

	i2c "github.com/d2r2/go-i2c"
	"github.com/d2r2/go-logger"
	l "github.com/kolobock/rockpi-quad-go/internal/logger"
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
	ssd1306SetIrefSelect      = 0xAD
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
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)

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
	l.Infof("[SSD1306] Initialized %dx%d display, buffer size: %d bytes", width, height, len(d.buffer))

	if err := d.init(); err != nil {
		i2cBus.Close()
		return nil, fmt.Errorf("failed to initialize SSD1306: %w", err)
	}

	return d, nil
}

// init initializes the SSD1306 display with proper configuration
func (d *SSD1306) init() error {
	cmds := []byte{
		ssd1306DisplayOff,
		ssd1306MemoryMode, 0x02,
		ssd1306SetDisplayClockDiv, 0x80,
		ssd1306SetMultiplex, byte(d.height - 1),
		ssd1306SetDisplayOffset, 0x00,
		ssd1306SetStartLine | 0x00,
		ssd1306SegRemap | 0x01,
		ssd1306ComScanDec,
	}

	if d.height == 32 {
		cmds = append(cmds, ssd1306SetComPins, 0x02)
	} else if d.height == 64 {
		cmds = append(cmds, ssd1306SetComPins, 0x12)
	}

	cmds = append(cmds,
		ssd1306SetPrecharge, 0xF1,
		ssd1306SetVcomDetect, 0x40,
		ssd1306SetContrast, 0xFF,
		ssd1306DisplayAllOnResume,
		ssd1306NormalDisplay,
		ssd1306SetIrefSelect, 0x30,
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
		if err := d.writeCmd(ssd1306SetLowColumn | 0x00); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetHighColumn | 0x00); err != nil {
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
	for i := 0; i < len(d.buffer); i++ {
		d.buffer[i] = 0
	}

	zeroPage := make([]byte, d.width+1)
	zeroPage[0] = 0x40

	for page := 0; page < d.height/8; page++ {
		if err := d.writeCmd(0xB0 | byte(page)); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetLowColumn | 0x00); err != nil {
			return err
		}
		if err := d.writeCmd(ssd1306SetHighColumn | 0x00); err != nil {
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
	d.SetDisplayOn(false)
	return d.i2c.Close()
}
