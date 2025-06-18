#!/bin/bash

# Dvarpala Cloud Installer Launcher
# One-click installation script for deploying dvarpala to AWS, GCP, or Azure
# Usage: curl -fsSL https://raw.githubusercontent.com/frigga-cloud/dvarpala/main/scripts/installation/install-dvarpala.sh | bash

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=================================================================="
echo -e "${BLUE}🚀 Dvarpala Cloud Installer${NC}"
echo -e "${BLUE}Deploy VPN servers to AWS, GCP, or Azure${NC}"
echo "=================================================================="
echo

# Check operating system
OS=""
ARCH=""

case "$(uname -s)" in
    Linux*)
        OS="linux"
        ;;
    Darwin*)
        OS="darwin"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        OS="windows"
        ;;
    *)
        echo -e "${RED}❌ Unsupported operating system: $(uname -s)${NC}"
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}❌ Unsupported architecture: $(uname -m)${NC}"
        exit 1
        ;;
esac

echo -e "${BLUE}🔍 Detected platform: ${OS}/${ARCH}${NC}"

# Check dependencies
check_dependencies() {
    local missing_deps=()
    
    # Check for curl
    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi
    
    # Check for git (optional but recommended)
    if ! command -v git &> /dev/null; then
        echo -e "${YELLOW}⚠️ Git not found. Consider installing for best experience.${NC}"
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        echo -e "${RED}❌ Missing required dependencies:${NC}"
        printf '   - %s\n' "${missing_deps[@]}"
        echo
        echo "Please install the missing dependencies and try again."
        exit 1
    fi
}

# Install Go if not present
install_go() {
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        echo -e "${GREEN}✅ Go found: version ${GO_VERSION}${NC}"
        return 0
    fi
    
    echo -e "${YELLOW}📦 Go not found. Installing Go...${NC}"
    
    GO_VERSION="1.21.5"
    GO_PACKAGE="go${GO_VERSION}.${OS}-${ARCH}"
    
    if [[ "$OS" == "windows" ]]; then
        GO_PACKAGE="${GO_PACKAGE}.zip"
    else
        GO_PACKAGE="${GO_PACKAGE}.tar.gz"
    fi
    
    # Download Go
    echo "   Downloading Go ${GO_VERSION}..."
    curl -fsSL "https://golang.org/dl/${GO_PACKAGE}" -o "/tmp/${GO_PACKAGE}"
    
    # Install Go
    if [[ "$OS" == "darwin" || "$OS" == "linux" ]]; then
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "/tmp/${GO_PACKAGE}"
        
        # Add to PATH
        export PATH=/usr/local/go/bin:$PATH
        
        # Update shell profile
        SHELL_PROFILE=""
        if [[ -f "$HOME/.bash_profile" ]]; then
            SHELL_PROFILE="$HOME/.bash_profile"
        elif [[ -f "$HOME/.bashrc" ]]; then
            SHELL_PROFILE="$HOME/.bashrc"
        elif [[ -f "$HOME/.zshrc" ]]; then
            SHELL_PROFILE="$HOME/.zshrc"
        fi
        
        if [[ -n "$SHELL_PROFILE" ]]; then
            if ! grep -q "/usr/local/go/bin" "$SHELL_PROFILE"; then
                echo 'export PATH=/usr/local/go/bin:$PATH' >> "$SHELL_PROFILE"
                echo -e "${GREEN}✅ Added Go to PATH in ${SHELL_PROFILE}${NC}"
            fi
        fi
        
    elif [[ "$OS" == "windows" ]]; then
        echo -e "${YELLOW}⚠️ Please install Go manually on Windows:${NC}"
        echo "   1. Download: https://golang.org/dl/${GO_PACKAGE}"
        echo "   2. Run the installer"
        echo "   3. Restart your terminal"
        echo "   4. Run this script again"
        exit 1
    fi
    
    # Cleanup
    rm -f "/tmp/${GO_PACKAGE}"
    
    echo -e "${GREEN}✅ Go ${GO_VERSION} installed successfully${NC}"
}

# Download dvarpala repository
download_dvarpala() {
    echo -e "${BLUE}📥 Downloading dvarpala...${NC}"
    
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    if command -v git &> /dev/null; then
        git clone --depth 1 https://github.com/frigga-cloud/dvarpala.git
    else
        # Fallback to downloading zip
        curl -fsSL https://github.com/frigga-cloud/dvarpala/archive/main.zip -o dvarpala.zip
        unzip -q dvarpala.zip
        mv dvarpala-main dvarpala
    fi
    
    cd dvarpala/scripts/installation
    echo -e "${GREEN}✅ Downloaded to: $(pwd)${NC}"
}

# Build cloud installer
build_installer() {
    echo -e "${BLUE}🔨 Building cloud installer...${NC}"
    
    # Check if we're in the right directory
    if [[ ! -f "cloud-installer.go" ]]; then
        echo -e "${RED}❌ Error: cloud-installer.go not found in current directory${NC}"
        echo "Current directory: $(pwd)"
        echo "Contents:"
        ls -la
        exit 1
    fi
    
    # Use existing build.sh script instead of duplicating logic
    if [[ -f "build.sh" ]]; then
        echo "Using existing build.sh script..."
        chmod +x build.sh
        ./build.sh
    else
        echo -e "${RED}❌ Error: build.sh not found${NC}"
        exit 1
    fi
}

# Main installation flow
main() {
    echo -e "${BLUE}🚀 Starting Dvarpala Cloud Installer setup...${NC}"
    echo
    
    check_dependencies
    install_go
    download_dvarpala
    build_installer
    
    echo
    echo -e "${GREEN}🎉 Setup completed successfully!${NC}"
    echo "================================================"
    echo
    echo -e "${BLUE}🚀 Ready to deploy dvarpala to the cloud!${NC}"
    echo
    echo "Usage:"
    echo "  ./dvarpala-installer                    # Interactive mode"
    echo "  ./dvarpala-installer -provider=aws     # AWS with prompts"
    echo "  ./dvarpala-installer -help             # Show all options"
    echo
    echo "Supported cloud providers:"
    echo "  • Amazon Web Services (AWS)"
    echo "  • Google Cloud Platform (GCP)"
    echo "  • Microsoft Azure"
    echo
    echo -e "${YELLOW}📖 For detailed documentation:${NC}"
    echo "   README.md in this directory"
    echo
    
    # Ask if user wants to run installer now
    echo -n "Would you like to run the installer now? (y/N): "
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        echo
        echo -e "${BLUE}🚀 Starting interactive installer...${NC}"
        
        # Verify binary exists before running
        if [[ -f "./dvarpala-installer" ]]; then
            ./dvarpala-installer
        else
            echo -e "${RED}❌ Error: dvarpala-installer binary not found${NC}"
            echo "Current directory: $(pwd)"
            echo "Available files:"
            ls -la
            exit 1
        fi
    else
        echo
        echo -e "${GREEN}✨ Setup complete. Run ./dvarpala-installer when ready.${NC}"
        echo "Location: $(pwd)/dvarpala-installer"
    fi
}

# Run main function
main "$@"