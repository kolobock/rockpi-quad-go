#!/bin/bash
# Pre-commit hook to run checks locally before pushing

set -e

echo "Running pre-commit checks..."

# Check formatting
echo "→ Checking Go formatting..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ The following files need formatting:"
    echo "$UNFORMATTED"
    echo "Run: gofmt -w ."
    exit 1
fi
echo "✓ All files properly formatted"

# Run go mod tidy check
echo "→ Checking go.mod and go.sum..."
go mod tidy
if ! git diff --exit-code go.mod go.sum > /dev/null; then
    echo "❌ go.mod or go.sum needs to be tidied"
    echo "Run: go mod tidy"
    git checkout go.mod go.sum
    exit 1
fi
echo "✓ go.mod and go.sum are clean"

# Run tests
echo "→ Running tests..."
go test -v ./pkg/... ./internal/config ./internal/logger
echo "✓ Tests passed"

# Check if golangci-lint is installed
if command -v golangci-lint &> /dev/null; then
    echo "→ Running golangci-lint..."
    # Only run on Linux or with GOOS=linux
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        golangci-lint run --timeout=5m
        echo "✓ Linter passed"
    else
        echo "⚠ Skipping linter on non-Linux platform (hardware dependencies)"
        echo "  Linter will run in CI on Linux"
    fi
else
    echo "⚠ golangci-lint not installed, skipping linter checks"
    echo "  Install: brew install golangci-lint (macOS) or see .github/CI.md"
fi

echo ""
echo "✅ All pre-commit checks passed!"
