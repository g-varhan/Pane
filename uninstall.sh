#!/usr/bin/env bash
# ============================================================================
#  Pane Uninstaller — https://github.com/pane-vmm/pane
#
#  Removes the Pane binary, libraries, runtime directories, and systemd
#  service from the system. Preserves VM images and snapshots by default.
#
#  Usage:
#    bash uninstall.sh [--purge] [--prefix /usr/local]
#
#    --purge     Also remove /var/lib/pane (images, snapshots, state)
#    --prefix    Installation prefix used at install time (default: /usr/local)
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
echo -e "${RED}${BOLD}"
cat <<'EOF'
  Pane Uninstaller
EOF
echo -e "${NC}"

# ── CLI flags ────────────────────────────────────────────────────────────────
PREFIX="/usr/local"
PURGE=false
YES=false

for arg in "$@"; do
  case "$arg" in
    --purge)      PURGE=true ;;
    -y|--yes)     YES=true ;;
    --prefix=*)   PREFIX="${arg#*=}" ;;
    -h|--help)
      echo "Usage: uninstall.sh [--purge] [-y|--yes] [--prefix=PATH]"
      echo "  --purge        Also remove /var/lib/pane (images, snapshots)"
      echo "  -y, --yes      Bypass the confirmation prompt"
      echo "  --prefix=PATH  Install prefix used at install time (default: /usr/local)"
      exit 0 ;;
  esac
done

BINDIR="$PREFIX/bin"
LIBDIR="$PREFIX/lib"
INCDIR="$PREFIX/include"

# ── Privilege helper ─────────────────────────────────────────────────────────
if [ "$(id -u)" -eq 0 ]; then SUDO=""; else SUDO="sudo"; fi

# ── Confirmation ──────────────────────────────────────────────────────────────
if [ "$YES" = false ]; then
  echo -e "${YELLOW}${BOLD}This will remove Pane from your system.${NC}"
  if [ "$PURGE" = true ]; then
    echo -e "${RED}${BOLD}--purge is set: ALL images and snapshots in /var/lib/pane will be deleted!${NC}"
  fi
  echo ""
  CONFIRM=""
  read -rp "  Continue? [y/N] " CONFIRM || true
  case "$CONFIRM" in
    [yY]|[yY][eE][sS]) : ;;
    *) echo "Aborted."; exit 0 ;;
  esac
fi

# ── 1. Stop and disable systemd service ──────────────────────────────────────
section "Stopping Pane Daemon"

if command -v systemctl &>/dev/null; then
  if systemctl is-active --quiet pane.service 2>/dev/null; then
    info "Stopping pane.service..."
    $SUDO systemctl stop pane.service
    ok "Service stopped"
  fi
  if systemctl is-enabled --quiet pane.service 2>/dev/null; then
    info "Disabling pane.service..."
    $SUDO systemctl disable pane.service
    ok "Service disabled"
  fi
  if [ -f /etc/systemd/system/pane.service ]; then
    $SUDO rm -f /etc/systemd/system/pane.service
    $SUDO systemctl daemon-reload
    ok "Service unit file removed"
  fi
else
  warn "systemd not found — skipping service removal"
fi

# Kill any remaining pane processes
if pgrep -x pane >/dev/null 2>&1; then
  info "Sending SIGTERM to running pane processes..."
  $SUDO pkill -TERM -x pane 2>/dev/null || true
  sleep 1
  # Force kill if still running
  if pgrep -x pane >/dev/null 2>&1; then
    $SUDO pkill -KILL -x pane 2>/dev/null || true
  fi
  ok "Pane processes terminated"
fi

# Remove PID and socket files
$SUDO rm -f /run/pane.sock /tmp/pane.sock /var/lib/pane/pane.pid /tmp/pane.pid
ok "Socket and PID files removed"

# ── 2. Remove installed binaries ──────────────────────────────────────────────
section "Removing Binaries"

remove_if_exists() {
  local file="$1"
  if [ -f "$file" ]; then
    $SUDO rm -f "$file"
    ok "Removed $file"
  else
    warn "Not found (already removed?): $file"
  fi
}

remove_if_exists "$BINDIR/pane"

# ── 3. Remove libraries and headers ──────────────────────────────────────────
section "Removing Libraries"

remove_if_exists "$LIBDIR/libpane_vmm.a"
remove_if_exists "$LIBDIR/libpane_core.a"
remove_if_exists "$INCDIR/pane_vmm.h"

command -v ldconfig &>/dev/null && $SUDO ldconfig 2>/dev/null || true

# ── 4. Remove runtime directories ────────────────────────────────────────────
section "Removing Runtime Directories"

$SUDO rm -rf /run/pane
ok "Removed /run/pane"

if [ "$PURGE" = true ]; then
  warn "Removing /var/lib/pane (all images and snapshots)..."
  $SUDO rm -rf /var/lib/pane
  ok "Removed /var/lib/pane"
else
  info "Preserving /var/lib/pane (images and snapshots)"
  info "Run with --purge to also remove VM data"
  # Still remove internal state, but not images/snapshots
  $SUDO rm -f /var/lib/pane/pane.pid
fi

# ── 5. Done ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}${BOLD}║   Pane successfully uninstalled.             ║${NC}"
echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════╝${NC}"
echo ""
if [ "$PURGE" = false ]; then
  echo -e "  ${YELLOW}Note:${NC} Your VM images and snapshots are preserved at ${BOLD}/var/lib/pane${NC}."
  echo -e "  To also remove them, re-run: ${CYAN}bash uninstall.sh --purge${NC}"
fi
echo ""
echo -e "  Thank you for using Pane! ⭐ https://github.com/pane-vmm/pane"
echo ""
