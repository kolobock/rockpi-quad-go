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
- 🚧 OLED display (TODO)
- 🚧 Key input handling (TODO)

## Installation

```bash
cd rockpi-quad-go
go build -o rockpi-quad ./cmd/rockpi-quad
sudo cp rockpi-quad /usr/bin/
sudo systemctl restart rockpi-quad
```

## Dependencies

```bash
go get gopkg.in/ini.v1
go get periph.io/x/conn/v3
go get periph.io/x/devices/v3
go get periph.io/x/host/v3
```

Or simply:
```bash
go mod download
```

## Configuration

Uses the same `/etc/rockpi-quad.conf` as the Python version.

## Environment Variables

- `HARDWARE_PWM=1` - Enable hardware PWM
- `PWM_CHIP=pwmchip0` - PWM chip device
- `PWM_CPU_FAN=0` - CPU fan PWM channel
- `PWM_TB_FAN=1` - Top/disk fan PWM channel
- `POLARITY=inversed` - PWM polarity

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
