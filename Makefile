.PHONY: build clean test install

BINARY_NAME=rockpi-quad
BUILD_DIR=build
INSTALL_DIR=/usr/bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rockpi-quad

build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rockpi-quad

clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test -v ./...

install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	sudo systemctl restart rockpi-quad

deps:
	go mod download
	go mod tidy

run:
	go run ./cmd/rockpi-quad
