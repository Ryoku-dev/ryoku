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

if ! docker run --rm -v "$repo:/repo:ro" "$image" /bin/sh -s <<'HARNESS'
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

raw_log=/tmp/installer-plan.typescript
log=/tmp/installer-plan.log
if ! runuser -u ryoku-ci -- env \
  HOME=/home/ryoku-ci USER=ryoku-ci LOGNAME=ryoku-ci TERM=xterm-256color \
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
if ! sh /repo/void/packages/resolve \
  --lane desktop --lane dev --session --build > "$resolved"; then
  fail "void package resolver rejected the installer lanes"
fi
[ -s "$resolved" ] || fail "void package resolver returned no packages"
: > "$installed"

while IFS= read -r line; do
  case $line in
    *'DRYRUN: sudo -n xbps-install -Sy '*)
      packages=${line#*DRYRUN: sudo -n xbps-install -Sy }
      ;;
    *'DRYRUN: xbps-install -Sy '*)
      packages=${line#*DRYRUN: xbps-install -Sy }
      ;;
    *)
      continue
      ;;
  esac
  for package in $packages; do
    case $package in
      -*) continue ;;
    esac
    printf '%s\n' "$package" >> "$installed"
    grep -Fxq "$package" "$resolved" ||
      fail "xbps install plan contains '$package', which the Void resolver did not return"
  done
done < "$log"

[ -s "$installed" ] || fail "dry-run log contains no xbps package install command"
for package in niri quickshell sddm dbus elogind turnstile socklog-void go; do
  grep -Fxq "$package" "$installed" ||
    fail "xbps install plan is missing required package '$package'"
done

if ! grep -Eq 'DRYRUN:.*\/repo\/ryoku\/shell\/deploy\.sh' "$log"; then
  fail "build step did not run ryoku/shell/deploy.sh"
fi
build_line=$(awk '/Building the Ryoku desktop from source/ { print NR; exit }' "$log")
driver_line=$(awk '/Setting up GPU drivers/ { print NR; exit }' "$log")
[ -n "$build_line" ] || fail "dry-run log has no source build step"
[ -n "$driver_line" ] || fail "dry-run log has no driver step"
[ "$build_line" -lt "$driver_line" ] ||
  fail "driver setup must run after the source build"

for service in socklog-unix nanoklogd dbus polkitd turnstiled sddm; do
  grep -Fq "ln -sfn /etc/sv/$service /etc/runit/runsvdir/default/$service" "$log" ||
    fail "session step did not link the $service runit service"
done

if grep -Eq 'DRYRUN:.*[[:space:]](pacman|systemctl)([[:space:]]|$)' "$log"; then
  fail "Void dry-run invoked an Arch/systemd command"
fi

printf '%s\n' 'void-installer-plan: OK'
HARNESS
then
  fail "Void container installer-plan check failed"
fi
