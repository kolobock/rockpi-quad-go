package oled

import (
	"fmt"
	"image"

	"github.com/d2r2/go-i2c"
	"github.com/d2r2/go-logger"
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
	ssd1306ExternalVcc        = 0x01
	ssd1306SwitchCapVcc       = 0x02

	ssd1306I2CAddr = 0x3C // Default I2C address for SSD1306
)

// SSD1306 represents an SSD1306 OLED display driver
type SSD1306 struct {
	i2c    *i2c.I2C
	width  int
	height int
}

// NewSSD1306 creates a new SSD1306 driver instance
// Tries common I2C bus numbers: 7, 1, 0
func NewSSD1306(width, height int) (*SSD1306, error) {
	// Disable verbose I2C logging
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)

	var i2cDev *i2c.I2C
	var err error

	// Try common I2C bus numbers for Rock Pi 4
	for _, bus := range []int{7, 1, 0} {
		i2cDev, err = i2c.NewI2C(ssd1306I2CAddr, bus)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open I2C: %w", err)
	}

	dev := &SSD1306{
		i2c:    i2cDev,
		width:  width,
		height: height,
	}

	// Initialize the display
	if err := dev.init(); err != nil {
		i2cDev.Close()
		return nil, fmt.Errorf("failed to initialize SSD1306: %w", err)
	}

	return dev, nil
}

// init initializes the SSD1306 display with proper configuration
func (d *SSD1306) init() error {
	// Initialization sequence for SSD1306
	cmds := []byte{
		ssd1306DisplayOff,
		// Address setting
		ssd1306MemoryMode, 0x10, // Page Addressing Mode (like CircuitPython default)
		// Resolution and layout
		ssd1306SetDisplayClockDiv, 0x80,
		ssd1306SetMultiplex, byte(d.height - 1),
		ssd1306SetDisplayOffset, 0x00,
		ssd1306SetStartLine | 0x00,
		ssd1306SegRemap | 0x01, // Column addr 127 mapped to SEG0
		ssd1306ComScanDec,      // Scan from COM[N] to COM0
	}

	// COM pins configuration depends on display height
	if d.height == 32 {
		cmds = append(cmds, ssd1306SetComPins, 0x02)
	} else if d.height == 64 {
		cmds = append(cmds, ssd1306SetComPins, 0x12)
	}

	// Timing and driving scheme
	cmds = append(cmds,
		ssd1306SetPrecharge, 0xF1, // Internal VCC
		ssd1306SetVcomDetect, 0x30, // 0.83*Vcc
		// Display
		ssd1306SetContrast, 0xFF, // Maximum
		ssd1306DisplayAllOnResume,
		ssd1306NormalDisplay,
		// Charge pump
		ssd1306ChargePump, 0x14, // Enable (internal VCC)
		ssd1306DisplayOn,
	)

	for _, cmd := range cmds {
		if err := d.i2c.WriteRegU8(0x00, cmd); err != nil {
			return err
		}
	}

	// Clear display on init
	return d.Clear()
}

// Display updates the OLED display with the contents of the image
func (d *SSD1306) Display(img *image.Gray) error {
	// Convert image to SSD1306 format (pages of 8 vertical pixels)
	// CircuitPython uses MVLSB format: bits are shifted left, MSB is top pixel
	buf := make([]byte, d.width*d.height/8)

	for page := 0; page < d.height/8; page++ {
		for x := 0; x < d.width; x++ {
			var b byte
			// Bits are packed with bit 0 = top pixel, bit 7 = bottom pixel
			// But we shift left, so we build from bit 7 down to bit 0
			for bit := 0; bit < 8; bit++ {
				b = b << 1
				y := page*8 + 7 - bit
				if img.GrayAt(x, y).Y > 128 {
					b |= 1
				}
			}
			buf[page*d.width+x] = b
		}
	}

	// Write data using Page Addressing Mode (like CircuitPython page_addressing=True)
	for page := 0; page < d.height/8; page++ {
		// Set page address (0xB0 + page number)
		if err := d.i2c.WriteRegU8(0x00, 0xB0|byte(page)); err != nil {
			return err
		}
		// Set column start address (low nibble)
		if err := d.i2c.WriteRegU8(0x00, byte(d.width%32)); err != nil {
			return err
		}
		// Set column start address (high nibble)
		if err := d.i2c.WriteRegU8(0x00, byte(0x10+d.width/32)); err != nil {
			return err
		}

		// Write page data
		pageData := make([]byte, d.width+1)
		pageData[0] = 0x40 // Co=0, D/C=1 (data mode)
		copy(pageData[1:], buf[page*d.width:(page+1)*d.width])
		if _, err := d.i2c.WriteBytes(pageData); err != nil {
			return err
		}
	}

	return nil
}

// Clear clears the display (turns all pixels off)
func (d *SSD1306) Clear() error {
	// Set column and page address range
	d.i2c.WriteRegU8(0x00, ssd1306ColumnAddr)
	d.i2c.WriteRegU8(0x00, 0)
	d.i2c.WriteRegU8(0x00, byte(d.width-1))
	d.i2c.WriteRegU8(0x00, ssd1306PageAddr)
	d.i2c.WriteRegU8(0x00, 0)
	d.i2c.WriteRegU8(0x00, byte((d.height/8)-1))

	// Create empty buffer
	buf := make([]byte, (d.width*d.height/8)+1)
	buf[0] = 0x40 // Data mode

	_, err := d.i2c.WriteBytes(buf)
	return err
}

// SetContrast sets the display contrast (0-255)
func (d *SSD1306) SetContrast(contrast byte) error {
	if err := d.i2c.WriteRegU8(0x00, ssd1306SetContrast); err != nil {
		return err
	}
	return d.i2c.WriteRegU8(0x00, contrast)
}

// SetDisplayOn turns the display on or off
func (d *SSD1306) SetDisplayOn(on bool) error {
	if on {
		return d.i2c.WriteRegU8(0x00, ssd1306DisplayOn)
	}
	return d.i2c.WriteRegU8(0x00, ssd1306DisplayOff)
}

// Close closes the I2C connection and turns off the display
func (d *SSD1306) Close() error {
	d.SetDisplayOn(false)
	return d.i2c.Close()
}
