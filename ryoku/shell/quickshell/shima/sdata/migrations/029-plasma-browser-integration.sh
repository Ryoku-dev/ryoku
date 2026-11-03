#!/usr/bin/env bash
# Migration: Install plasma-browser-integration for reliable browser MPRIS.
# Browser media controls and artwork depend on the native host plus browser-side
# integration; without it some browser players expose incomplete MPRIS sessions.

MIGRATION_ID="029-plasma-browser-integration"
MIGRATION_TITLE="Browser media integration"
MIGRATION_DESCRIPTION="Installs plasma-browser-integration so browser media exposes reliable MPRIS sessions, controls, and artwork."
MIGRATION_TARGET_FILE="system packages"
MIGRATION_REQUIRED=true

migration_check() {
  ! command -v plasma-browser-integration-host >/dev/null 2>&1
}

migration_preview() {
  echo -e "${STY_YELLOW}  Package: plasma-browser-integration${STY_RST}"
  echo ""
  echo "  Adds the native browser integration host used for browser MPRIS sessions."
  echo "  Restart open browsers after installing so they pick up the native host."
}

migration_diff() {
  if command -v plasma-browser-integration-host >/dev/null 2>&1; then
    echo "plasma-browser-integration-host is already available."
  else
    echo "plasma-browser-integration-host is missing."
  fi
}

migration_apply() {
  if ! migration_check; then
    return 0
  fi

  if command -v pacman >/dev/null 2>&1; then
    echo "Installing plasma-browser-integration..."
    pkg_sudo pacman -S --needed --noconfirm plasma-browser-integration 2>/dev/null || {
      echo -e "${STY_YELLOW}Could not auto-install plasma-browser-integration.${STY_RST}"
      echo -e "${STY_YELLOW}Install manually: sudo pacman -S plasma-browser-integration${STY_RST}"
      return 1
    }
  elif command -v xbps-install >/dev/null 2>&1; then
    echo "Installing plasma-browser-integration..."
    pkg_sudo xbps-install -S -y plasma-browser-integration 2>/dev/null || {
      echo -e "${STY_YELLOW}Could not auto-install plasma-browser-integration.${STY_RST}"
      echo -e "${STY_YELLOW}Install manually: sudo xbps-install -S plasma-browser-integration${STY_RST}"
      return 1
    }
  elif command -v apt >/dev/null 2>&1; then
    echo "Installing plasma-browser-integration..."
    pkg_sudo apt install -y plasma-browser-integration 2>/dev/null || {
      echo -e "${STY_YELLOW}Could not auto-install plasma-browser-integration.${STY_RST}"
      echo -e "${STY_YELLOW}Install manually: sudo apt install plasma-browser-integration${STY_RST}"
      return 1
    }
  elif command -v dnf >/dev/null 2>&1; then
    echo "Installing plasma-browser-integration..."
    pkg_sudo dnf install -y plasma-browser-integration 2>/dev/null || {
      echo -e "${STY_YELLOW}Could not auto-install plasma-browser-integration.${STY_RST}"
      echo -e "${STY_YELLOW}Install manually: sudo dnf install plasma-browser-integration${STY_RST}"
      return 1
    }
  else
    echo -e "${STY_YELLOW}Install plasma-browser-integration using your package manager.${STY_RST}"
    return 1
  fi

  if ! command -v plasma-browser-integration-host >/dev/null 2>&1; then
    echo -e "${STY_YELLOW}plasma-browser-integration-host still not found after install.${STY_RST}"
    echo -e "${STY_YELLOW}Restart your browser/session or install it manually.${STY_RST}"
    return 1
  fi

  echo "  Restart open browsers so they pick up plasma-browser-integration."
}
