.PHONY: all build dev test lint clean install-deps frontend

# Variables
APP_NAME := nightscout-tray
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

# Default target
all: lint test build

# Install dependencies
install-deps:
	go mod download
	cd frontend && npm ci

# Build frontend
frontend:
	cd frontend && npm run build

# Development mode
dev:
	wails3 dev

# Build for current platform
build: frontend
	wails3 build

# Build for Windows
build-windows: frontend
	wails3 task windows:build

# Build for Linux (requires Linux/WSL/Docker)
build-linux: frontend
	wails3 task linux:build

# Build for macOS (requires macOS)
build-darwin: frontend
	wails3 task darwin:build

# Build for all platforms (requires cross-compilation setup with Docker)
build-all: frontend
	@echo "Building for all platforms requires Docker for cross-compilation"
	@echo "Use 'make build-windows', 'make build-linux', or 'make build-darwin' for specific platforms"
	wails3 task windows:build

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint code
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	goimports -w $(GO_FILES)

# Clean build artifacts
clean:
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/node_modules
	rm -f coverage.out coverage.html

# Install the application (Linux)
install: build
	sudo cp build/bin/$(APP_NAME) /usr/local/bin/
	sudo cp build/linux/$(APP_NAME).desktop /usr/share/applications/
	sudo cp build/appicon.png /usr/share/icons/hicolor/256x256/apps/$(APP_NAME).png
	sudo gtk-update-icon-cache /usr/share/icons/hicolor/

# Uninstall the application (Linux)
uninstall:
	sudo rm -f /usr/local/bin/$(APP_NAME)
	sudo rm -f /usr/share/applications/$(APP_NAME).desktop
	sudo rm -f /usr/share/icons/hicolor/256x256/apps/$(APP_NAME).png

# Generate mocks for testing
mocks:
	go generate ./...

# Check for security issues
security:
	gosec ./...

# Update dependencies
update-deps:
	go get -u ./...
	go mod tidy
	cd frontend && npm update

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Run lint, test, and build"
	@echo "  install-deps - Install Go and npm dependencies"
	@echo "  frontend     - Build frontend only"
	@echo "  dev          - Run in development mode"
	@echo "  build        - Build for current platform"
	@echo "  build-all    - Build for all platforms"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  clean        - Clean build artifacts"
	@echo "  install      - Install application (Linux)"
	@echo "  uninstall    - Uninstall application (Linux)"
	@echo "  security     - Run security checks"
	@echo "  update-deps  - Update dependencies"
