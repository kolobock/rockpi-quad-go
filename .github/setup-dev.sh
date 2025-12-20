#!/bin/bash
# Setup development environment with git hooks

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Setting up development environment..."

# Link pre-commit hook
if [ -f "$SCRIPT_DIR/hooks/pre-commit.sh" ]; then
    echo "→ Installing pre-commit hook..."
    ln -sf "$SCRIPT_DIR/hooks/pre-commit.sh" "$PROJECT_ROOT/.git/hooks/pre-commit"
    chmod +x "$PROJECT_ROOT/.git/hooks/pre-commit"
    echo "✓ Pre-commit hook installed"
else
    echo "⚠ Pre-commit hook not found"
fi

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo ""
    echo "⚠ golangci-lint is not installed"
    echo "  Install it for better code quality checks:"
    echo ""
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "  brew install golangci-lint"
    else
        echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin"
    fi
    echo ""
else
    echo "✓ golangci-lint is installed"
fi

# Download dependencies
echo "→ Downloading Go dependencies..."
go mod download
echo "✓ Dependencies downloaded"

echo ""
echo "✅ Development environment setup complete!"
echo ""
echo "Available commands:"
echo "  make test       - Run non-hardware tests"
echo "  make test-all   - Compile all tests for Linux"
echo "  make build      - Build for ARM64"
echo "  golangci-lint run - Run linter"
echo ""
