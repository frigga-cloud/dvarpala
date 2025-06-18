#!/bin/bash

# Build script for Dvarpala Cloud Installer
# This script builds the cloud installer binary for distribution

set -euo pipefail

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔨 Building Dvarpala Cloud Installer${NC}"
echo "======================================"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.19+ first."
    echo "   Download from: https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.19"

if ! printf '%s\n%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V -C; then
    echo "❌ Go version $GO_VERSION is too old. Please upgrade to Go $REQUIRED_VERSION or later."
    exit 1
fi

echo "✅ Go version: $GO_VERSION"

# Initialize Go module if not exists
if [[ ! -f go.mod ]]; then
    echo "🔧 Initializing Go module..."
    go mod init dvarpala-cloud-installer
fi

# Get dependencies
echo "📦 Getting dependencies..."
go mod tidy

# Build for current platform
echo "🏗️ Building cloud installer..."
go build -o dvarpala-installer -ldflags "-X main.Version=$(date +%Y%m%d-%H%M%S)" cloud-installer.go

# Make executable
chmod +x dvarpala-installer

echo -e "${GREEN}✅ Build completed successfully!${NC}"
echo
echo "📄 Usage:"
echo "  ./dvarpala-installer                    # Interactive mode"
echo "  ./dvarpala-installer -provider=aws     # AWS with prompts"
echo "  ./dvarpala-installer -config=config.json  # Use configuration file"
echo
echo "📖 For detailed documentation, see:"
echo "   README.md in this directory"
echo
echo "🚀 To start installation:"
echo "   ./dvarpala-installer"