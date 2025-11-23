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
- ✅ OLED display with page cycling
- ✅ Environment file loading (/etc/rockpi-quad.env)
- 🚧 Key input handling (TODO)

## Installation

```bash
# Build the binary
cd rockpi-quad-go
GOOS=linux GOARCH=arm64 go build -o rockpi-quad-go ./cmd/rockpi-quad

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
go get github.com/warthog618/go-gpiocdev
go get gopkg.in/ini.v1
go get golang.org/x/image
```

Or simply:
```bash
go mod download
```

## Configuration

The application uses two configuration files:

### `/etc/rockpi-quad.conf`
Main configuration file (same format as Python version) containing:
- Fan temperature thresholds and PWM levels
- OLED display settings (rotation, temperature unit)
- Disk monitoring configuration
- Key/button behavior settings

### `/etc/rockpi-quad.env`
Environment configuration file (same as Python version) containing hardware-specific settings:
- I2C pins for OLED (SDA, SCL, OLED_RESET)
- Button GPIO configuration
- Fan control GPIO settings
- SATA LED indicators
- PWM configuration

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

## Testing

```bash
go test ./...
```

## Project Structure

```
rockpi-quad-go/
├── cmd/
│   └── rockpi-quad/          # Main application entry point
│       └── main.go
├── internal/
│   ├── config/               # Configuration loading
│   │   └── config.go
│   ├── fan/                  # Fan control logic
│   │   └── fan.go
│   ├── oled/                 # OLED display (TODO)
│   │   └── oled.go
│   └── disk/                 # Disk temperature monitoring
│       └── disk.go
└── pkg/
    └── pwm/                  # PWM hardware interface
        └── pwm.go
```

## License

Same as the original Python version.
