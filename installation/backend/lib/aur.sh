#!/usr/bin/env bash
# shellcheck shell=bash
# build the AUR set in the target. base system has no AUR helper, so we
# bootstrap yay (clone from the AUR, fall back to the GitHub release binary
# when the AUR is unreachable), then install system/packages/aur.packages.
#
# makepkg refuses to run as root, so the build runs as the unpriv user via
# runuser, with a one-shot NOPASSWD sudo drop-in (removed at the end) so
# makepkg/yay can drive pacman without a TTY prompt. chroot is lent the live
# resolv.conf for DNS while building, then restored.
#
# best-effort: offline install or a failed build = warn + return 0, so the
# install still completes (user bootstraps later). runs after the bootloader
# step while /mnt is still mounted; emits no @@RYOKU_STEP.

# boot menu integrity: these AUR packages own the Limine boot menu (the UKI hook +
# snapshot sync behind the tool-managed /boot/limine.conf). the VM run proved yay's
# single end-of-run batch lets one dead download wedge the whole set, so these MUST
# install first, each time-bounded. aur.packages still carries them so the offline
# mirror builds them too; this list just front-runs them.
CRITICAL=(limine-mkinitcpio-hook limine-snapper-sync)

# ryoku_aur_is_critical PKG: is PKG in the boot-critical set (installed first,
# so it is dropped from the best-effort remainder batch).
ryoku_aur_is_critical() {
  local p=$1 c
  for c in "${CRITICAL[@]}"; do [[ $p == "$c" ]] && return 0; done
  return 1
}

ryoku_aur() {
  local u=$RYOKU_USERNAME
  local aur_file="$RYOKU_REPO/system/packages/aur.packages"

  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: bootstrap yay, install boot-critical (%s) first under per-package timeouts, then the rest of %s best-effort under a global timeout, as %s' "${CRITICAL[*]}" "$aur_file" "$u"
    return 0
  fi
  if [[ -n ${RYOKU_SKIP_AUR:-} ]]; then
    log "AUR: RYOKU_SKIP_AUR set, skipping the AUR set (bootstrap it later)"
    return 0
  fi
  if [[ ${RYOKU_ONLINE:-1} != 1 ]]; then
    log "AUR: offline install, the AUR set is installed from the baked [offline] repo by ryoku_offline_aur (lib/offline.sh); nothing to build here"
    return 0
  fi
  [[ -f $aur_file ]] || { log 'AUR: no %s, skipping' "$aur_file"; return 0; }

  local -a pkgs=()
  mapfile -t pkgs < <(ryoku_drop_from < <(grep -vE '^[[:space:]]*(#|$)' "$aur_file"))
  (( ${#pkgs[@]} )) || { log "AUR: nothing left to build after the installer's app/browser choices; skipping"; return 0; }

  log 'AUR: bootstrapping yay and building %d package(s); this can take several minutes' "${#pkgs[@]}"

  # one-shot NOPASSWD sudo so makepkg/yay can call pacman without a prompt.
  printf '%s ALL=(ALL) NOPASSWD: ALL\n' "$u" >/mnt/etc/sudoers.d/99-ryoku-aur-build
  chmod 0440 /mnt/etc/sudoers.d/99-ryoku-aur-build
  # security: always remove the passwordless-sudo drop-in from the TARGET, even
  # on an early return, so the installed system never ships with NOPASSWD sudo.
  trap 'rm -f /mnt/etc/sudoers.d/99-ryoku-aur-build 2>/dev/null' RETURN

  # chroot needs DNS to reach the AUR and GitHub. only set it when the
  # target has none yet, and undo exactly that after.
  local made_resolv=0
  if [[ ! -e /mnt/etc/resolv.conf ]] && cp -L /etc/resolv.conf /mnt/etc/resolv.conf 2>/dev/null; then
    made_resolv=1
  fi

  # 1. bootstrap yay as the user. HOME forced; runuser keeps root's env.
  if arch-chroot /mnt runuser -u "$u" -- env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" bash -s <<'BOOTSTRAP'; then
set -e
command -v yay >/dev/null && exit 0
sudo pacman -S --needed --noconfirm base-devel git curl >/dev/null
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
ok=0
for attempt in 1 2 3; do
  rm -rf "$work/yay-bin"
  if git clone --depth 1 https://aur.archlinux.org/yay-bin.git "$work/yay-bin"; then ok=1; break; fi
  echo "git clone of yay-bin failed (attempt $attempt/3), retrying in 5s..."
  sleep 5
done
if (( ok == 1 )); then
  cd "$work/yay-bin"
  makepkg -si --noconfirm
else
  echo "AUR unreachable, falling back to the yay GitHub release binary..."
  cd "$work"
  url=$(curl -fsSL https://api.github.com/repos/Jguer/yay/releases/latest \
    | grep -oE '"browser_download_url": *"[^"]*x86_64\.tar\.gz"' | head -1 \
    | sed 's/.*: *"\(.*\)"/\1/')
  [[ -n $url ]] || { echo "could not determine the yay release URL" >&2; exit 1; }
  curl -fsSL "$url" -o yay.tar.gz
  tar xzf yay.tar.gz
  bin=$(find . -maxdepth 2 -type f -name yay | head -1)
  [[ -n $bin ]] || { echo "yay binary not found in the tarball" >&2; exit 1; }
  sudo install -m0755 "$bin" /usr/local/bin/yay
fi
command -v yay >/dev/null
BOOTSTRAP
    # boot-critical FIRST, each in its OWN time-bounded transaction. the VM run
    # proved yay's single end-of-run batch lets one dead download (voxtype's) wedge
    # forever, so the limine hooks below never landed and the box booted with no
    # tool-managed menu. front-run them, each bounded, so one hung build can't
    # starve the others.
    local crit rest=()
    for crit in "${CRITICAL[@]}"; do
      log 'AUR: installing boot-critical %s (<= 25 min)' "$crit"
      if ! arch-chroot /mnt runuser -u "$u" -- env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" \
        timeout 1500 yay -S --noconfirm --needed "$crit"; then
        log 'AUR: WARNING, boot-critical %s timed out or failed to build and was skipped; the boot menu integration may be incomplete ('\''ryoku doctor'\'' converges it once online). Continuing.' "$crit"
      fi
    done
    # everything else in one best-effort, globally time-bounded batch. the
    # criticals are dropped from it (already handled); a stall here cannot hold
    # the install hostage.
    for crit in "${pkgs[@]}"; do
      ryoku_aur_is_critical "$crit" || rest+=("$crit")
    done
    if (( ${#rest[@]} )); then
      log 'AUR: installing the remaining %d package(s): %s (<= 40 min)' "${#rest[@]}" "${rest[*]}"
      arch-chroot /mnt runuser -u "$u" -- env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" \
        timeout 2400 yay -S --noconfirm --needed "${rest[@]}" \
        || log 'AUR: WARNING, some of these timed out or did not build and were skipped: %s (continuing; install them later with yay)' "${rest[*]}"
    fi
  else
    log "AUR: warning, yay bootstrap failed; the AUR set was not installed"
  fi

  # restore the target's sudo + resolv.conf state.
  rm -f /mnt/etc/sudoers.d/99-ryoku-aur-build
  if (( made_resolv == 1 )); then
    rm -f /mnt/etc/resolv.conf
  fi
  return 0
}

# browser_desktop_keys NAME: the .desktop ids a shipped browser may install
# under. zen ships a few across its AUR rebuilds; the repo browsers are fixed.
browser_desktop_keys() {
  case "$1" in
    zen) echo "zen.desktop zen-browser.desktop app.zen_browser.zen.desktop" ;;
    chromium) echo "chromium.desktop" ;;
    firefox) echo "firefox.desktop" ;;
  esac
}

# ryoku_default_browser points the target user's default web browser at the
# installer's pick, but only when that browser actually landed. It sets the
# http/https scheme handlers and HTML files through xdg-mime, then records the
# same pick as desktop.apps.browser in ~/.config/ryoku/desktop.json so the
# launcher and the shell agree with xdg. Install-time only: `ryoku update`
# never repoints a browser, so an existing box keeps its choice.
ryoku_default_browser() {
  local u="$RYOKU_USERNAME" br="${RYOKU_BROWSER:-firefox}" desk="" d
  if [[ -n ${RYOKU_DRYRUN:-} ]]; then
    log 'DRYRUN: set %s as the default web browser (xdg + desktop.apps.browser) for %s if installed' "$br" "$u"
    return 0
  fi
  for d in $(browser_desktop_keys "$br"); do
    if [[ -f /mnt/usr/share/applications/$d ]]; then desk=$d; break; fi
  done
  if [[ -z $desk ]]; then
    log "default browser: %s not installed; leaving the browser default unchanged" "$br"
    return 0
  fi
  if arch-chroot /mnt runuser -u "$u" -- env "HOME=/home/$u" "USER=$u" "LOGNAME=$u" \
    xdg-mime default "$desk" x-scheme-handler/http x-scheme-handler/https text/html 2>/dev/null; then
    log 'default browser: set %s (%s) for %s' "$br" "$desk" "$u"
  else
    log 'default browser: warning, could not set %s for %s (continuing)' "$br" "$u"
  fi
  # The desktop role: same key the Hub's Default Apps page writes. jq is in
  # the base set; a merge failure (corrupt store) is logged and skipped, the
  # ryoku-app fallback still resolves the command by name.
  local store="/mnt/home/$u/.config/ryoku/desktop.json"
  if command -v jq >/dev/null 2>&1; then
    local next
    if [[ -s $store ]]; then
      next=$(jq --arg b "$(browser_role_cmd "$br")" '. * {desktop:{apps:{browser:$b}}}' "$store") || next=""
    else
      mkdir -p "$(dirname "$store")"
      next=$(jq -n --arg b "$(browser_role_cmd "$br")" '{desktop:{apps:{browser:$b}}}') || next=""
    fi
    if [[ -n $next ]]; then
      printf '%s\n' "$next" >"$store"
      # runs after deploy.sh's chown pass: hand the seeded file back to the user.
      chown "$u:$u" "$store" 2>/dev/null || true
      log 'browser role: desktop.apps.browser -> %s' "$(browser_role_cmd "$br")"
    else
      log 'browser role: skip (could not merge %s)' "$store"
    fi
  fi
  return 0
}

# browser_role_cmd KEY is the command stored in the desktop browser role. Zen's
# desktop IDs vary across builds, but its executable remains `zen`; an absent or
# unknown key falls back to Firefox, matching the installer's default.
browser_role_cmd() {
  case "$1" in
    zen) echo zen ;;
    chromium) echo chromium ;;
    *) echo firefox ;;
  esac
}
