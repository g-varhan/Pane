#!/usr/bin/env bash
# ============================================================================
#  Pane Installer — https://github.com/g-varhan/Pane
#
#  Supports: Arch Linux · Debian · Ubuntu · Fedora · RHEL/Rocky/Alma ·
#            openSUSE Leap/Tumbleweed
#  Requires: Linux x86_64, KVM-capable host, internet access
#
#  Usage (one-liner):
#    curl -fsSL https://raw.githubusercontent.com/g-varhan/Pane/main/install.sh | bash
#
#  Or locally:
#    bash install.sh [--no-daemon] [--prefix /usr/local]
# ============================================================================
set -euo pipefail

# ── Colours ─────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info()    { echo -e "${BLUE}${BOLD}  ▸${NC} $*"; }
ok()      { echo -e "${GREEN}${BOLD}  ✓${NC} $*"; }
warn()    { echo -e "${YELLOW}${BOLD}  ⚠${NC}  $*"; }
die()     { echo -e "${RED}${BOLD}  ✗ ERROR:${NC} $*" >&2; exit 1; }
section() { echo -e "\n${CYAN}${BOLD}══ $* ══${NC}"; }

# ── Banner ───────────────────────────────────────────────────────────────────
echo -e "${CYAN}${BOLD}"
cat <<'EOF'
  ██████╗  █████╗ ███╗   ██╗███████╗
  ██╔══██╗██╔══██╗████╗  ██║██╔════╝
  ██████╔╝███████║██╔██╗ ██║█████╗
  ██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝
  ██║     ██║  ██║██║ ╚████║███████╗
  ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝
EOF
echo -e "${NC}${BOLD}  Installer v0.2.0  —  SQLite, but for VMs${NC}\n"

# ── CLI flags ────────────────────────────────────────────────────────────────
PREFIX="/usr/local"
INSTALL_DAEMON=true
SKIP_DEPS=false

for arg in "$@"; do
  case "$arg" in
    --no-daemon)   INSTALL_DAEMON=false ;;
    --skip-deps)   SKIP_DEPS=true ;;
    --prefix=*)    PREFIX="${arg#*=}" ;;
    -h|--help)
      echo "Usage: install.sh [--no-daemon] [--skip-deps] [--prefix=PATH]"
      echo "  --no-daemon    Do not install/enable the systemd service"
      echo "  --skip-deps    Skip system package installation"
      echo "  --prefix=PATH  Install prefix (default: /usr/local)"
      exit 0 ;;
  esac
done

BINDIR="$PREFIX/bin"
LIBDIR="$PREFIX/lib"
INCDIR="$PREFIX/include"

# ── Privilege helper ─────────────────────────────────────────────────────────
if [ "$(id -u)" -eq 0 ]; then SUDO=""; else SUDO="sudo"; fi

need_sudo() {
  if [ "$(id -u)" -ne 0 ] && ! command -v sudo &>/dev/null; then
    die "Root privileges are required. Please run as root or install sudo."
  fi
}

# ── 1. Platform checks ────────────────────────────────────────────────────────
section "Platform Verification"

[ "$(uname -s)" = "Linux" ] || die "Pane only supports Linux hosts."
[ "$(uname -m)" = "x86_64" ] || die "Pane currently only supports x86_64."

# KVM check
if [ ! -e /dev/kvm ]; then
  warn "/dev/kvm not found. VMs will run without hardware acceleration."
  warn "Enable KVM in BIOS/UEFI (Intel VT-x / AMD-V) for best performance."
else
  ok "KVM is available (/dev/kvm)"
  # Ensure current user can access /dev/kvm
  if [ ! -r /dev/kvm ] || [ ! -w /dev/kvm ]; then
    warn "Current user cannot access /dev/kvm. Adding to 'kvm' group..."
    need_sudo
    $SUDO usermod -aG kvm "$USER" || warn "Failed to add user to kvm group. You may need to do this manually."
    warn "You must log out and back in (or run 'newgrp kvm') for group changes to take effect."
  fi
fi

# Check required tools
for tool in git make curl; do
  command -v "$tool" &>/dev/null || die "Required tool '$tool' is not installed."
done
ok "Required tools available"

# ── 2. Detect distro & install dependencies ───────────────────────────────────
section "Dependency Installation"

install_deps_arch() {
  info "Detected: Arch Linux"
  $SUDO pacman -Sy --needed --noconfirm \
    base-devel liburing clang llvm lld \
    rust go qemu-system-x86 \
    protobuf grpc
}

install_deps_debian() {
  info "Detected: Debian / Ubuntu"
  $SUDO apt-get update -qq
  $SUDO apt-get install -y --no-install-recommends \
    build-essential clang llvm lld liburing-dev \
    rustc cargo golang-go \
    qemu-system-x86 \
    protobuf-compiler libprotobuf-dev
  # Ensure Go is recent enough (Debian ships old versions)
  GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
  GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
  GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
  if [ "${GO_MAJOR:-0}" -lt 1 ] || { [ "${GO_MAJOR:-0}" -eq 1 ] && [ "${GO_MINOR:-0}" -lt 21 ]; }; then
    warn "System Go ($GO_VERSION) is too old. Installing Go 1.22 via snap..."
    $SUDO snap install go --classic 2>/dev/null || \
      warn "snap not available — please install Go >= 1.21 manually from https://go.dev/dl/"
  fi
}

install_deps_fedora() {
  info "Detected: Fedora / RHEL / Rocky / Alma"
  $SUDO dnf install -y \
    gcc gcc-c++ make clang llvm lld liburing-devel \
    rust cargo golang \
    qemu-system-x86 \
    protobuf-compiler protobuf-devel
}

install_deps_opensuse() {
  info "Detected: openSUSE"
  $SUDO zypper install -y \
    gcc gcc-c++ make clang llvm lld liburing-devel \
    rust cargo go \
    qemu-x86 \
    protobuf-devel
}

if [ "$SKIP_DEPS" = false ]; then
  need_sudo
  if [ -f /etc/arch-release ]; then
    install_deps_arch
  elif [ -f /etc/debian_version ]; then
    install_deps_debian
  elif [ -f /etc/fedora-release ] || [ -f /etc/redhat-release ]; then
    install_deps_fedora
  elif [ -f /etc/SUSE-brand ] || [ -f /etc/os-release ] && grep -q "openSUSE" /etc/os-release 2>/dev/null; then
    install_deps_opensuse
  else
    warn "Unknown Linux distribution. Attempting to continue — you may need to install:"
    warn "  liburing-dev, clang, rust, cargo, golang, make, git, qemu-system-x86"
  fi
  ok "Dependencies installed"
else
  warn "Skipping dependency installation (--skip-deps)"
fi

# Validate Go/Rust after installation
command -v go &>/dev/null   || die "Go compiler not found. Install Go >= 1.21 from https://go.dev/dl/"
command -v cargo &>/dev/null || die "Rust/Cargo not found. Install from https://rustup.rs/"
ok "Go $(go version | awk '{print $3}') and Rust $(rustc --version | awk '{print $2}') ready"

# ── 3. Clone or use local source ──────────────────────────────────────────────
section "Source Setup"

# Detect if running from an already-cloned repo
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
if [ -f "$SCRIPT_DIR/go.mod" ] && grep -q "^module pane" "$SCRIPT_DIR/go.mod" 2>/dev/null; then
  BUILD_DIR="$SCRIPT_DIR"
  info "Using existing source at $BUILD_DIR"
  CLEANUP_BUILD=false
else
  BUILD_DIR=$(mktemp -d)
  info "Cloning Pane repository to $BUILD_DIR ..."
  git clone --depth 1 https://github.com/g-varhan/Pane.git "$BUILD_DIR"
  CLEANUP_BUILD=true
fi
ok "Source ready"

# ── 4. Build pane-vmm (C static library) ─────────────────────────────────────
section "Building pane-vmm (C / KVM layer)"

make -C "$BUILD_DIR/pane-vmm" clean
make -C "$BUILD_DIR/pane-vmm" CFLAGS="-Wall -Wextra -O2"
ok "pane-vmm built → libpane_vmm.a"

# ── 5. Build pane-core (Rust orchestration library) ──────────────────────────
section "Building pane-core (Rust orchestration)"

cd "$BUILD_DIR/pane-core"
cargo build --release 2>&1 | tail -5
cd "$BUILD_DIR"
ok "pane-core built → libpane_core.a"

# ── 6. Build pane CLI (Go, CGo-linked) ───────────────────────────────────────
section "Building pane CLI (Go)"

cd "$BUILD_DIR"

# Set CGo library search paths so the Go linker finds our freshly built libs.
export CGO_CFLAGS="-I$BUILD_DIR/pane-vmm/include -I$BUILD_DIR/pane-api/ffi"
export CGO_LDFLAGS="-L$BUILD_DIR/pane-vmm -L$BUILD_DIR/pane-core/target/release -lpane_vmm -lpane_core -luring"
export CGO_ENABLED=1

go build \
  -ldflags="-s -w -X main.Version=v0.2.0 -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o "$BUILD_DIR/pane" \
  ./pane-cli/
ok "pane CLI built"

# ── 7. Install binaries & libraries ──────────────────────────────────────────
section "Installing to $PREFIX"

need_sudo
$SUDO mkdir -p "$BINDIR" "$LIBDIR" "$INCDIR"

$SUDO install -m 755 "$BUILD_DIR/pane" "$BINDIR/pane"
ok "Installed pane → $BINDIR/pane"

$SUDO install -m 644 "$BUILD_DIR/pane-vmm/include/pane_vmm.h" "$INCDIR/pane_vmm.h"
$SUDO install -m 644 "$BUILD_DIR/pane-vmm/libpane_vmm.a"      "$LIBDIR/libpane_vmm.a"
$SUDO install -m 644 "$BUILD_DIR/pane-core/target/release/libpane_core.a" "$LIBDIR/libpane_core.a"
ok "Installed libraries → $LIBDIR"

# Refresh linker cache if ldconfig exists
command -v ldconfig &>/dev/null && $SUDO ldconfig || true

# ── 8. Runtime directories & state ───────────────────────────────────────────
section "Runtime Setup"

$SUDO mkdir -p /var/lib/pane/images /var/lib/pane/snapshots /run/pane
$SUDO chmod 755 /var/lib/pane /run/pane
ok "Runtime directories created (/var/lib/pane, /run/pane)"

# ── 9. Optional: systemd service ─────────────────────────────────────────────
if [ "$INSTALL_DAEMON" = true ] && command -v systemctl &>/dev/null; then
  section "Installing systemd Service"

  $SUDO tee /etc/systemd/system/pane.service >/dev/null <<UNIT
[Unit]
Description=Pane VM Lifecycle Daemon
Documentation=https://github.com/g-varhan/Pane
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=$BINDIR/pane daemon start
ExecStop=/bin/kill -TERM \$MAINPID
Restart=on-failure
RestartSec=2
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pane
# Security hardening
ProtectSystem=full
PrivateTmp=true
NoNewPrivileges=true
# Allow KVM and network access
SupplementaryGroups=kvm
AmbientCapabilities=CAP_NET_ADMIN CAP_SYS_ADMIN

[Install]
WantedBy=multi-user.target
UNIT

  $SUDO systemctl daemon-reload
  $SUDO systemctl enable pane.service
  ok "Systemd service installed and enabled (pane.service)"
  info "Start now: sudo systemctl start pane"
elif [ "$INSTALL_DAEMON" = false ]; then
  warn "Skipping daemon installation (--no-daemon)"
else
  warn "systemd not found — skipping service installation"
fi

# ── 10. Cleanup ───────────────────────────────────────────────────────────────
if [ "${CLEANUP_BUILD:-false}" = true ]; then
  rm -rf "$BUILD_DIR"
  ok "Cleaned up build directory"
fi

# ── 11. Done ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}${BOLD}║   Pane v0.2.0 installed successfully! 🎉    ║${NC}"
echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}Quick start:${NC}"
echo -e "    ${CYAN}pane daemon start${NC}          # Start the VM daemon"
echo -e "    ${CYAN}pane pull alpine${NC}           # Download Alpine Linux"
echo -e "    ${CYAN}pane run alpine${NC}            # Boot it"
echo -e "    ${CYAN}pane ps${NC}                    # List running VMs"
echo -e "    ${CYAN}pane snapshot <id>${NC}         # Snapshot a VM"
echo -e "    ${CYAN}pane fork <id> <new-id>${NC}   # Clone a VM"
echo ""
echo -e "  ${BOLD}Documentation:${NC}"
echo -e "    https://github.com/g-varhan/Pane/blob/main/docs/README.md"
echo ""
echo -e "  ${BOLD}To uninstall:${NC}"
echo -e "    ${CYAN}curl -fsSL https://raw.githubusercontent.com/g-varhan/Pane/main/uninstall.sh | bash${NC}"
echo ""
