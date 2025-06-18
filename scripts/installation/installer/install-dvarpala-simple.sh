#!/bin/bash

# Dvarpala Cloud Installer Launcher (Simplified)
# One-click installation script for deploying dvarpala to AWS, GCP, or Azure
# Usage: curl -fsSL https://raw.githubusercontent.com/frigga-cloud/dvarpala/main/scripts/installation/install-dvarpala-simple.sh | bash

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=================================================================="
echo -e "${BLUE}🚀 Dvarpala Cloud Installer (Simple)${NC}"
echo -e "${BLUE}Deploy VPN servers to AWS, GCP, or Azure${NC}"
echo "=================================================================="
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed.${NC}"
    echo
    echo "Please install Go first:"
    echo "  • Download from: https://golang.org/dl/"
    echo "  • Or use your package manager:"
    echo "    - macOS: brew install go"
    echo "    - Ubuntu/Debian: sudo apt install golang-go"
    echo "    - CentOS/RHEL: sudo yum install golang"
    echo
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✅ Go found: version ${GO_VERSION}${NC}"

# Download dvarpala repository
echo -e "${BLUE}📥 Downloading dvarpala...${NC}"

TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

if command -v git &> /dev/null; then
    git clone --depth 1 https://github.com/frigga-cloud/dvarpala.git
else
    # Check if curl is available
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}❌ Neither git nor curl is available. Please install one of them.${NC}"
        exit 1
    fi
    
    # Fallback to downloading zip
    curl -fsSL https://github.com/frigga-cloud/dvarpala/archive/main.zip -o dvarpala.zip
    
    # Check if unzip is available
    if ! command -v unzip &> /dev/null; then
        echo -e "${RED}❌ unzip is not available. Please install unzip or git.${NC}"
        exit 1
    fi
    
    unzip -q dvarpala.zip
    mv dvarpala-main dvarpala
fi

cd dvarpala/scripts/installation
echo -e "${GREEN}✅ Downloaded to: $(pwd)${NC}"

# Verify cloud-installer.go exists
if [[ ! -f "cloud-installer.go" ]]; then
    echo -e "${RED}❌ Error: cloud-installer.go not found${NC}"
    echo "Current directory: $(pwd)"
    echo "Contents:"
    ls -la
    exit 1
fi

# Run the cloud installer directly
echo
echo -e "${GREEN}🎉 Ready to deploy dvarpala to the cloud!${NC}"
echo "================================================"
echo
echo -e "${BLUE}🚀 Starting Dvarpala Cloud Installer...${NC}"
echo

# Ask if user wants to run installer now
echo -n "Would you like to start the installation? (Y/n): "
read -r response
if [[ "$response" =~ ^[Nn]$ ]]; then
    echo
    echo -e "${YELLOW}Installation cancelled.${NC}"
    echo "To run later, execute:"
    echo "  cd $(pwd)"
    echo "  go run cloud-installer.go"
    exit 0
fi

echo
echo -e "${BLUE}🚀 Starting interactive installer...${NC}"
echo

# Initialize Go module if needed
if [[ ! -f go.mod ]]; then
    go mod init dvarpala-cloud-installer
    go mod tidy
fi

# Run the cloud installer directly
go run cloud-installer.go

echo
echo -e "${GREEN}✨ Installation process completed!${NC}"