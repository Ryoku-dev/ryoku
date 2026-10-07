#!/usr/bin/env bash
#
# ryoku-shell bootstrap: fetch and run the standalone Ryoku desktop installer
# on an existing Arch machine. Kept deliberately dumb: every real decision
# lives in the ryoku-shell-install binary this script downloads.
#
#   curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku-arch/main/ryoku-shell-installer/install.sh | bash
#
# args after `bash -s --` are forwarded to the installer (--yes, --dry-run).
# RYOKU_SHELL_REF picks the git ref to fetch the installer and payload from.
# unstable-dev also puts the box on the unstable (testing) package channel:
#
#   curl -fsSL https://raw.githubusercontent.com/ryoku-dev/ryoku-arch/unstable-dev/ryoku-shell-installer/install.sh | RYOKU_SHELL_REF=unstable-dev bash
set -euo pipefail

main() {
  local ref="${RYOKU_SHELL_REF:-main}"
  local raw="https://raw.githubusercontent.com/ryoku-dev/ryoku-arch/${ref}/ryoku-shell-installer"

  # English on purpose: this bootstrap runs before any Ryoku catalog exists on
  # the box to translate from; the ryoku-shell-install binary it fetches does that.
  say() { printf '\033[38;2;242;86;35m==>\033[0m %s\n' "$*"; }
  die() {
    printf 'ryoku-shell: %s\n' "$*" >&2
    exit 1
  }

  [[ $(id -u) -ne 0 ]] || die "run as your normal user, not root (sudo is used when needed)"

  # NixOS needs a nix-based engine; that work is parked (archived flake),
  # so refuse honestly instead of dying on the package-manager guard below.
  [[ ! -e /etc/NIXOS ]] || die "NixOS is not supported yet; use the flake instead"

  local ryoku_family
  if command -v pacman > /dev/null 2>&1; then
    ryoku_family=arch
  elif command -v apt-get > /dev/null 2>&1; then
    ryoku_family=debian
  else
    die "unsupported distribution: Ryoku installs on Arch-based and Debian-based systems"
  fi
  [[ $(uname -m) == x86_64 ]] || die "Ryoku ships x86_64 builds only"
  # the binary refuses non-systemd boots much later (session + services are
  # systemd units); saying it here spares Artix users the download.
  [[ -d /run/systemd/system ]] || die "this installer needs systemd (Artix and other non-systemd inits are not supported)"
  command -v curl > /dev/null 2>&1 || die "curl is required"

  # warn-only on derivatives: the package manager is what actually matters.
  if [[ -r /etc/os-release ]]; then
    # shellcheck source=/dev/null
    . /etc/os-release
    case "${ID:-} ${ID_LIKE:-}" in
      *arch*|*debian*) ;;
      *) say "warning: ${PRETTY_NAME:-unknown distro} is not recognised; continuing as ${ryoku_family}" ;;
    esac
  fi
  if [[ $ryoku_family == debian ]]; then
    say "Debian detected: the desktop is built from source, which takes a few minutes"
  fi

  # global, not local: the EXIT trap fires after main returns, when a local is
  # gone and set -u would turn every finished run into exit 1.
  work="$(mktemp -d)"
  trap 'rm -rf "$work"' EXIT

  say "// fetching  Ryoku shell installer (${ref})"
  curl -fsSL --retry 3 -o "$work/ryoku-shell-install" "$raw/ryoku-shell-install"
  curl -fsSL --retry 3 -o "$work/ryoku-shell-install.sha256" "$raw/ryoku-shell-install.sha256"
  (cd "$work" && sha256sum --check --quiet ryoku-shell-install.sha256) \
    || die "checksum mismatch on the downloaded installer; try again"
  chmod +x "$work/ryoku-shell-install"

  say "// starting  Ryoku shell installer"
  local rc=0
  # piped stdin (curl | bash) is useless to a TUI; hand it the real terminal.
  if [[ ! -t 0 && -r /dev/tty ]]; then
    RYOKU_SHELL_REF="$ref" "$work/ryoku-shell-install" "$@" < /dev/tty || rc=$?
  else
    RYOKU_SHELL_REF="$ref" "$work/ryoku-shell-install" "$@" || rc=$?
  fi
  return "$rc"
}

main "$@"
