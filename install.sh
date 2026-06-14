#!/bin/sh
set -e

# Pane installer script for Arch, Debian/Ubuntu, and Fedora (x86_64)
# Run via: curl -fsSL https://raw.githubusercontent.com/g-varhan/Pane/main/install.sh | sh

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0;3c' # No Color
BOLD='\033[1m'

echo "${BLUE}${BOLD}=== Installing Pane (v0.1.0) ===${NC}"

# 1. Platform verification
OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" != "Linux" ]; then
    echo "${RED}Error: Pane only supports Linux hosts.${NC}" >&2
    exit 1
fi

if [ "$ARCH" != "x86_64" ]; then
    echo "${RED}Error: Pane currently only supports x86_64 architecture.${NC}" >&2
    exit 1
fi

# Check for sudo/root permissions
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
else
    SUDO=""
fi

# Detect Package Manager and Install Dependencies
if [ -f /etc/arch-release ]; then
    echo "${BLUE}Detected Arch Linux. Installing dependencies via pacman...${NC}"
    $SUDO pacman -S --needed --noconfirm liburing clang rust go make git
elif [ -f /etc/debian_version ]; then
    echo "${BLUE}Detected Debian/Ubuntu. Installing dependencies via apt...${NC}"
    $SUDO apt-get update
    $SUDO apt-get install -y liburing-dev clang rustc cargo golang-go make git
elif [ -f /etc/fedora-release ] || [ -f /etc/redhat-release ]; then
    echo "${BLUE}Detected Fedora/RHEL. Installing dependencies via dnf...${NC}"
    $SUDO dnf install -y liburing-devel clang rust cargo golang make git
else
    echo "${RED}Unsupported Linux distribution. Please manually install dependencies: liburing, clang, rust, go, make, git.${NC}" >&2
    exit 1
fi

# Create a temporary directory for building
BUILD_DIR=$(mktemp -d)
echo "${BLUE}Cloning Pane repository...${NC}"
git clone --depth 1 https://github.com/g-varhan/Pane.git "$BUILD_DIR"
cd "$BUILD_DIR"

# 2. Build VMM C Static Library
echo "${BLUE}Building pane-vmm...${NC}"
make -C pane-vmm clean
make -C pane-vmm CFLAGS="-Wall -Wextra -Werror -O2"

# 3. Build Rust Core Orchestrator
echo "${BLUE}Building pane-core...${NC}"
cd pane-core
cargo build --release
cd ..

# 4. Build Go gRPC Server Daemon
echo "${BLUE}Building pane-api daemon...${NC}"
cd pane-api
CGO_ENABLED=1 go build -ldflags="-s -w" -o pane-api main.go
cd ..

# 5. Installing Binaries and Libraries
echo "${BLUE}Installing Pane to system directories...${NC}"
$SUDO mkdir -p /usr/local/bin
$SUDO mkdir -p /usr/local/include
$SUDO mkdir -p /usr/local/lib

$SUDO cp pane-api/pane-api /usr/local/bin/pane-api
$SUDO cp pane-vmm/include/pane_vmm.h /usr/local/include/pane_vmm.h
$SUDO cp pane-vmm/libpane_vmm.a /usr/local/lib/libpane_vmm.a

# Clean up
rm -rf "$BUILD_DIR"

echo "${GREEN}${BOLD}✓ Pane v0.1.0 installed successfully!${NC}"
echo "To start the Pane gRPC daemon, run:"
echo "  ${BOLD}pane-api${NC}"
echo ""
echo "For tutorials and guides, see: https://github.com/g-varhan/Pane/blob/main/getstarted.md"
