#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
image=${RYOKU_VOID_IMAGE:-ghcr.io/void-linux/void-glibc:latest}

fail() {
  printf 'void-installer-plan: FAIL: %s\n' "$*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || fail "docker is required"
[[ -x $repo/ryoku-shell-installer/ryoku-shell-install ]] ||
  fail "committed ryoku-shell-install binary is missing or not executable"
[[ -x $repo/void/packages/resolve ]] ||
  fail "void/packages/resolve is missing or not executable"

if ! docker run --rm -i -v "$repo:/repo:ro" "$image" /bin/sh -s <<'HARNESS'
set -eu

fail() {
  printf 'void-installer-plan: FAIL: %s\n' "$*" >&2
  if [ -f /tmp/installer-plan.log ]; then
    printf '%s\n' '--- dry-run log ---' >&2
    cat /tmp/installer-plan.log >&2
  fi
  exit 1
}

xbps-install -Sy util-linux sudo shadow >/dev/null 2>&1 ||
  fail "could not install util-linux, sudo, and shadow in the Void container"
useradd -m -s /bin/sh ryoku-ci || fail "could not create the non-root test user"
printf '%s\n' 'ryoku-ci ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/ryoku-ci
chmod 0440 /etc/sudoers.d/ryoku-ci

mkdir -p /tmp/ryoku-test-keys
printf '%s\n' 'test key plist' > /tmp/ryoku-test-keys/test-key.plist

raw_log=/tmp/installer-plan.typescript
log=/tmp/installer-plan.log
if ! runuser -u ryoku-ci -- env \
  HOME=/home/ryoku-ci USER=ryoku-ci LOGNAME=ryoku-ci TERM=xterm-256color \
  RYOKU_XBPS_REPO=https://repo.test/void \
  RYOKU_XBPS_KEYRING_DIR=/tmp/ryoku-test-keys \
  script -q -e -c "/repo/ryoku-shell-installer/ryoku-shell-install --yes --dry-run --payload /repo" "$raw_log"; then
  [ -f "$raw_log" ] && cat "$raw_log" >&2
  fail "the committed installer dry-run failed as a non-root user"
fi

escape=$(printf '\033')
tr -d '\r' < "$raw_log" | sed "s/${escape}\[[0-9;]*m//g" > "$log"

if ! grep -Eq 'system:[[:space:]]+Void' "$log"; then
  fail "preflight did not accept the Void host"
fi
if ! grep -Fq '/etc/runit/runsvdir/default/' "$log"; then
  fail "the dry-run did not select the runit plan"
fi

resolved=/tmp/resolved.packages
installed=/tmp/planned.packages
# The installer's hardware lane depends on the container's detected GPU, so
# compare its package transaction with the union of every hardware lane.
if ! sh /repo/void/packages/resolve \
  --lane desktop --lane hardware:amd --lane hardware:intel \
  --lane hardware:nvidia --lane hardware:vm --session > "$resolved"; then
  fail "void package resolver rejected the installer lanes"
fi
[ -s "$resolved" ] || fail "void package resolver returned no packages"

key_line=$(awk '/install -Dm644 \/tmp\/ryoku-test-keys\/test-key\.plist \/var\/db\/xbps\/keys\/test-key\.plist/ { print NR; exit }' "$log")
repo_line=$(awk '/DRYRUN: write \/etc\/xbps\.d\/20-ryoku\.conf/ { print NR; exit }' "$log")
sync_line=$(awk -v after="$repo_line" 'NR > after && /xbps-install -S$/ { print NR; exit }' "$log")
[ -n "$key_line" ] || fail "repo step did not seed the XBPS trust key"
[ -n "$repo_line" ] || fail "repo step did not write /etc/xbps.d/20-ryoku.conf"
[ -n "$sync_line" ] || fail "repo step did not sync XBPS after writing the repository"
[ "$key_line" -lt "$repo_line" ] && [ "$repo_line" -lt "$sync_line" ] ||
  fail "XBPS trust key and repository must be installed before the first Ryoku sync"

enabler_count=$(grep -Ec 'DRYRUN: sudo -n xbps-install -Sy void-repo-[^ ]+( void-repo-[^ ]+)*$' "$log" || :)
[ "$enabler_count" -eq 1 ] ||
  fail "expected exactly one void-repo enabler transaction, got $enabler_count"
package_command=$(grep -E 'DRYRUN: sudo -n xbps-install -Sy .*ryoku-keyring' "$log" || :)
[ "$(printf '%s\n' "$package_command" | grep -c .)" -eq 1 ] ||
  fail "expected exactly one Ryoku package transaction"
packages=${package_command#*DRYRUN: sudo -n xbps-install -Sy }
: > "$installed"
for package in $packages; do
  printf '%s\n' "$package" >> "$installed"
  case $package in
    ryoku-keyring|ryoku-desktop|ryoku-desktop-niri) ;;
    *) grep -Fxq "$package" "$resolved" ||
      fail "XBPS package transaction contains '$package', which the resolver did not return" ;;
  esac
done

for package in ryoku-keyring ryoku-desktop ryoku-desktop-niri niri quickshell sddm dbus elogind turnstile socklog-void; do
  grep -Fxq "$package" "$installed" ||
    fail "XBPS package transaction is missing required package '$package'"
done
for package in go cmake; do
  if grep -Fxq "$package" "$installed"; then
    fail "packaged install unexpectedly includes source-build package '$package'"
  fi
done

if grep -Eq 'DRYRUN:.*xbps-install.* (go|cmake|base-devel)( |$)' "$log"; then
  fail "packaged Void plan installs a source-build toolchain package"
fi

if grep -Eq 'Building the Ryoku desktop from source|\/ryoku\/shell\/deploy\.sh|Installing Ryoku fonts and cursors' "$log"; then
  fail "packaged Void plan still contains a source-install step"
fi
package_line=$(awk '/Installing the Ryoku desktop/ { print NR; exit }' "$log")
driver_line=$(awk '/Setting up GPU drivers/ { print NR; exit }' "$log")
[ -n "$package_line" ] || fail "dry-run log has no package step"
[ -n "$driver_line" ] || fail "dry-run log has no driver step"
[ "$package_line" -lt "$driver_line" ] ||
  fail "driver setup must run after the package transaction"

for service in socklog-unix nanoklogd dbus polkitd turnstiled sddm; do
  grep -Fq "ln -sfn /etc/sv/$service /etc/runit/runsvdir/default/$service" "$log" ||
    fail "session step did not link the $service runit service"
done

system_session_line=$(awk '/ryoku-host session ensure --system/ { print NR; exit }' "$log")
user_session_line=$(awk '/ryoku-host session ensure --user/ { print NR; exit }' "$log")
[ -n "$system_session_line" ] && [ -n "$user_session_line" ] ||
  fail "session step did not run both ryoku-host session ensure modes"
[ "$system_session_line" -lt "$user_session_line" ] ||
  fail "system session provisioning must run before user provisioning"
grep -Fq 'ryoku-host session fix-wrappers --backup-dir' "$log" ||
  fail "session step did not retain the Turnstile wrapper repair"
grep -Fq 'verify Ryoku XBPS repo, trusted key, ryoku-desktop packages, Turnstile, niri session' "$log" ||
  fail "verify step does not cover the packaged Void contract"

if grep -Eq 'DRYRUN:.*[[:space:]](pacman|systemctl)([[:space:]]|$)' "$log"; then
  fail "Void dry-run invoked an Arch/systemd command"
fi

printf '%s\n' 'void-installer-plan: OK'
HARNESS
then
  fail "Void container installer-plan check failed"
fi
