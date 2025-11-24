.PHONY: build clean test install build-amd64 install-amd64

BINARY_NAME=rockpi-quad-go
BUILD_DIR=build
INSTALL_DIR=/usr/bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rockpi-quad-go

build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/rockpi-quad-go
clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test -v ./pkg/... ./internal/config

test-linux:
	GOOS=linux go test -v ./...

install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	sudo systemctl restart rockpi-quad-go

install-amd64: build-amd64
	sudo systemctl stop rockpi-quad-go
	sudo cp $(BUILD_DIR)/$(BINARY_NAME)-amd64 $(INSTALL_DIR)/$(BINARY_NAME)
	sudo systemctl restart rockpi-quad-go

deps:
	go mod download
	go mod tidy

run:
	go run ./cmd/rockpi-quad-go
