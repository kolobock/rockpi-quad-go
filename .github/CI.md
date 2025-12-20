# CI/CD Documentation

## GitHub Actions Workflow

The project uses GitHub Actions for continuous integration with the following jobs:

### 1. **Lint** (`lint`)
- Runs `golangci-lint` with comprehensive linter configuration
- Configuration in `.golangci.yml`
- Includes `govet` among 25+ other linters
- Checks code quality, style, and potential bugs

### 2. **Format Check** (`format`)
- Verifies all Go files are properly formatted with `gofmt`
- Fails if any files need formatting
- Run locally: `gofmt -w .`

### 3. **Go Mod Verify** (`mod-verify`)
- Downloads all dependencies
- Verifies `go.mod` and `go.sum` integrity
- Checks that files are tidy (no uncommitted changes after `go mod tidy`)
- Run locally: `go mod tidy && go mod verify`

### 4. **Build** (`build`)
- Cross-compiles for:
  - `linux/arm64` (target platform)
  - `linux/amd64` (development/testing)
- Uploads build artifacts
- Run locally: `GOOS=linux GOARCH=arm64 go build -o build/rockpi-quad-go-arm64 ./cmd/rockpi-quad-go`

### 5. **Test** (`test`)
- Runs unit tests for non-hardware packages
- Packages tested: `./pkg/...`, `./internal/config`, `./internal/logger`
- Generates coverage report
- Compiles all tests for `linux/arm64` to verify they build correctly
- Run locally: `go test -v -race -coverprofile=coverage.txt ./pkg/... ./internal/config ./internal/logger`

### 6. **Test on ARM64** (`test-arm64`)
- Uses QEMU to emulate ARM64 platform
- Runs tests in Docker container on ARM64 architecture
- Alternative: Can use self-hosted ARM64 runner (commented in workflow)
- Run locally: `docker buildx build --platform linux/arm64 -f .github/workflows/Dockerfile.test -t rockpi-test:arm64 .`

> **Note**: `go vet` is not run as a separate job because it's already included in golangci-lint.

## Running Tests Locally

### Non-hardware tests (macOS/Linux/Windows):
```bash
go test -v ./pkg/... ./internal/config ./internal/logger
```

### All tests with Linux cross-compilation:
```bash
GOOS=linux go test -c ./...
```

### Using Docker for ARM64 tests:
```bash
docker buildx build --platform linux/arm64 -f .github/workflows/Dockerfile.test -t rockpi-test:arm64 .
docker run --rm --platform linux/arm64 rockpi-test:arm64
```

### Using Makefile:
```bash
make test          # Non-hardware tests
make test-all      # Compile all tests for Linux
```

## Self-Hosted ARM64 Runner (Optional)

For actual hardware testing, you can set up a self-hosted ARM64 runner:

1. On your ARM64 device, install GitHub Actions runner:
   ```bash
   mkdir actions-runner && cd actions-runner
   curl -o actions-runner-linux-arm64.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-arm64-2.311.0.tar.gz
   tar xzf ./actions-runner-linux-arm64.tar.gz
   ./config.sh --url https://github.com/kolobock/rockpi-quad-go --token YOUR_TOKEN
   sudo ./svc.sh install
   sudo ./svc.sh start
   ```

2. Add labels: `self-hosted`, `linux`, `arm64`

3. Uncomment in `.github/workflows/ci.yml`:
   ```yaml
   test-arm64:
     runs-on: [self-hosted, linux, arm64]
   ```

## Linter Configuration

The project uses `golangci-lint` with custom configuration in `.golangci.yml`:

### Enabled Linters:
- **errcheck**: Check for unchecked errors
- **gosec**: Security issues
- **govet**: Suspicious constructs
- **ineffassign**: Ineffectual assignments
- **staticcheck**: Advanced static analysis
- **stylecheck**: Style issues
- **unused**: Unused code
- And many more...

### Run linter locally:
```bash
golangci-lint run --timeout=5m
```

Or install and run:
```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Run
golangci-lint run
```

## Coverage Reports

Coverage is automatically uploaded to Codecov (when configured).

Generate local coverage:
```bash
go test -coverprofile=coverage.txt -covermode=atomic ./pkg/... ./internal/config ./internal/logger
go tool cover -html=coverage.txt
```

## Troubleshooting

### Tests fail on macOS
Hardware-dependent packages (button, disk, fan, oled) require Linux GPIO/I2C libraries. Use Docker or `GOOS=linux` for compilation checks only.

### Docker ARM64 emulation is slow
This is expected with QEMU emulation. For faster tests, use:
1. Native ARM64 hardware with self-hosted runner
2. GitHub's hosted ARM64 runners (when available)
3. Skip emulated tests for non-hardware code

### Go version mismatch
Update `.github/workflows/ci.yml` and local environment to match the version in `go.mod`.
