# RockPi Quad - Go Implementation

A Go reimplementation of the RockPi SATA HAT fan and OLED controller.

## Features

- ✅ Dual PWM fan control (CPU + Disk fans)
- ✅ Linear temperature interpolation
- ✅ Separate temperature thresholds for CPU and disk fans
- ✅ Disk temperature monitoring via SMART
- ✅ Syslog support
- ✅ Inversed polarity support
- ✅ Minimum duty cycle threshold (7%)
- ✅ SSD1306 OLED display (128x32) with multi-font support
- ✅ Multiple display pages (system info, fan speed, disk usage, network I/O, disk I/O, disk temps)
- ✅ Configurable page cycling (10-second intervals)
- ✅ 180° display rotation support
- ✅ Button input handling (click/double-click/long-press)
- ✅ Configurable button actions (slider, switch, poweroff, reboot, custom commands)
- ✅ Environment file loading (/etc/rockpi-quad.env)

## Installation

```bash
# Build the binary
cd rockpi-quad-go
GOOS=linux GOARCH=arm64 go build -o rockpi-quad-go ./cmd/rockpi-quad
# or 
make build-arm64

# Install on Rock Pi 4
sudo mkdir -p /usr/bin/rockpi-quad
sudo cp rockpi-quad-go /usr/bin/rockpi-quad/
sudo cp rockpi-quad-go.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable rockpi-quad-go
sudo systemctl start rockpi-quad-go

# Check status
sudo systemctl status rockpi-quad-go
```

## Configuration Files

The application uses the same configuration files as the Python version:
- `/etc/rockpi-quad.conf` - Application settings
- `/etc/rockpi-quad.env` - Hardware/GPIO configuration

No changes to existing config files are needed.

## Dependencies

```bash
go get github.com/d2r2/go-i2c
go get github.com/d2r2/go-logger
go get github.com/warthog618/go-gpiocdev
go get github.com/golang/freetype
go get gopkg.in/ini.v1
go get golang.org/x/image
```

Or simply:
```bash
go mod download
```

Or `make deps`

## Configuration

The application uses two configuration files:

### `/etc/rockpi-quad.conf`
Main configuration file (same format as Python version) containing:
- Fan temperature thresholds and PWM levels
- OLED display settings (rotation, temperature unit, enabled/disabled)
- Disk monitoring configuration (mount points for usage/I/O, temperature disks)
- Network interface configuration
- Key/button behavior settings (click, double-click, long-press actions)
- Timing settings for button detection

### `/etc/rockpi-quad.env`
Environment configuration file (same as Python version) containing hardware-specific settings:
- I2C pins for OLED (SDA, SCL, OLED_RESET)
- Button GPIO configuration
- Fan control GPIO settings
- SATA LED indicators
- PWM configuration

**Note:** Both files are shared with the Python version - no changes needed to switch between implementations.

## OLED Display Pages

The OLED displays the following information pages in rotation:

1. **System Info Page 0**: Uptime, CPU temperature, IP address
2. **System Info Page 1**: Fan speeds (CPU & Disk), CPU load, Memory usage
3. **Disk Usage**: Root partition and data disk usage percentages (sorted: sda, sdb, sdc, sdd)
4. **Network I/O**: RX/TX rates for configured network interfaces
5. **Disk I/O**: Read/Write rates for configured disks
6. **Disk Temperatures**: Temperature readings for SATA disks

Display features:
- **Multi-font support**: Uses DejaVu Sans Mono Bold TTF in sizes 10, 11, 12, and 14
- **Proper Unicode**: Supports degree symbol (°) and other special characters
- **Two-column layout**: Efficient use of 128x32 pixel display
- **Auto-detection**: Automatically detects SATA disks for temperature monitoring
- **Configurable**: Can be enabled/disabled, rotated 180°, and switch between Celsius/Fahrenheit

## Button Actions

The button supports three types of presses with configurable actions:

- **Single Click** (`click`): Default action is `slider` (advance to next OLED page)
- **Double Click** (`twice`): Default action is `switch` (toggle fan on/off)
- **Long Press** (`press`): Default action is `poweroff` (system shutdown)

Configurable actions in `/etc/rockpi-quad.conf`:
```ini
[key]
click = slider      # Options: slider, switch, poweroff, reboot, none, or custom shell command
twice = switch
press = poweroff
```

Timing configuration:
```ini
[time]
twice = 0.7         # Double-click detection window (seconds)
press = 1.8         # Long-press threshold (seconds)
```

**Note:** Both files are shared with the Python version - no changes needed to switch between implementations.

## Environment Variables

The following environment variables are loaded from `/etc/rockpi-quad.env`:

**OLED Display:**
- `SDA` - I2C data pin (e.g., I2C7_SDA)
- `SCL` - I2C clock pin (e.g., I2C7_SCL)
- `OLED_RESET` - OLED reset GPIO (e.g., GPIO4_D2)

**Button:**
- `BUTTON_CHIP` - GPIO chip number
- `BUTTON_LINE` - GPIO line number

**Fan Control:**
- `FAN_CHIP` - GPIO chip for fan control
- `FAN_LINE` - GPIO line for fan control
- `HARDWARE_PWM` - Set to "1" to enable hardware PWM

**PWM (when HARDWARE_PWM=1):**
- `PWM_CHIP` - PWM chip device (default: pwmchip0)
- `PWM_CPU_FAN` - CPU fan PWM channel
- `PWM_TB_FAN` - Top/disk fan PWM channel
- `POLARITY` - PWM polarity (normal/inversed)

**SATA LEDs:**
- `SATA_CHIP` - GPIO chip for SATA LEDs
- `SATA_LINE_1` - First SATA LED GPIO line
- `SATA_LINE_2` - Second SATA LED GPIO line

## Advantages over Python Version

- **Lower memory footprint** (~5MB vs ~30MB)
- **Better CPU efficiency** (compiled binary)
- **Single binary deployment** (no dependencies to install)
- **No runtime dependencies** (no Python interpreter needed)
- **Faster startup time** (no module loading)
- **Built-in concurrency** with goroutines
- **Static typing** catches errors at compile time

## Building for ARM64 (Rock Pi 4)

```bash
GOOS=linux GOARCH=arm64 go build -o rockpi-quad ./cmd/rockpi-quad
```

## Project Structure

```
rockpi-quad-go/
├── cmd/
│   └── rockpi-quad-go/       # Main application entry point
│       └── main.go
├── internal/
│   ├── config/               # Configuration loading
│   │   └── config.go
│   ├── fan/                  # Fan control logic
│   │   └── fan.go
│   ├── button/               # Button input handling
│   │   └── button.go
│   ├── oled/                 # OLED display controller
│   │   ├── oled.go           # Display controller
│   │   ├── pages.go          # Page definitions and data
│   │   └── ssd1306.go        # SSD1306 I2C driver
│   └── disk/                 # Disk temperature monitoring
│       └── disk.go
├── pkg/
│   └── pwm/                  # PWM hardware interface
│       └── pwm.go
└── fonts/
    └── DejaVuSansMono-Bold.ttf  # TrueType font for OLED
```

## Testing

The project includes comprehensive unit tests for core functionality:

```bash
# Run tests (macOS/Linux compatible tests only)
make test

# Run all tests (requires Linux environment)
make test-linux

# Run tests with coverage
go test -cover ./pkg/... ./internal/config

# Run specific package tests
go test -v ./pkg/pwm
go test -v ./internal/config
```

### Test Coverage

- **pkg/pwm**: PWM duty cycle calculation and sysfs operations
- **internal/config**: Configuration file loading and defaults
- **internal/fan**: Fan speed calculation (linear and non-linear modes)
- **internal/button**: Button event type handling
- **internal/oled**: Display rendering, page generation, and image rotation
- **internal/disk**: Device name parsing and temperature monitoring

Note: Some tests require a Linux environment with GPIO hardware support to run fully.

## License

Same as the original Python version.
