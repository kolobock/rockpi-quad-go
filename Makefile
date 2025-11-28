.PHONY: build clean test install

BINARY_NAME=rockpi-quad-go
BUILD_DIR=build
INSTALL_DIR=/usr/bin

build:
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/rockpi-quad-go
clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test -v ./pkg/... ./internal/config

test-linux:
	GOOS=linux go test -v ./...

install: build
	sudo systemctl stop rockpi-quad-go
	sudo cp $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(INSTALL_DIR)/$(BINARY_NAME)
	sudo systemctl restart rockpi-quad-go

deps:
	go mod download
	go mod tidy

run:
	go run ./cmd/rockpi-quad-go
