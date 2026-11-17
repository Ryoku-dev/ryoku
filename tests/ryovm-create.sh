#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
engine="$here/../ryoku/apps/ryovm/bin/ryovm"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/home"
for cmd in bash mkdir mktemp setsid tail jq ls grep basename sed tr rm; do
    ln -s "$(command -v "$cmd")" "$work/bin/$cmd"
done
cat >"$work/bin/quickget" <<'EOF'
#!/bin/bash
name="$1-$2${3:+-$3}"
mkdir -p "$name"
case "$TEST_MODE" in
    disk) printf 'keep disk\n' >"$name/disk.qcow2" ;;
    conf)
        printf 'guest_os="linux"\niso="%s/missing.iso"\n' "$name" >"$name.conf" ;;
    success)
        printf 'media\n' >"$name/install.iso"
        printf 'guest_os="linux"\niso="%s/install.iso"\n' "$name" >"$name.conf"
        exit 0 ;;
esac
printf 'URL could not be resolved\n' >&2
exit 7
EOF
chmod +x "$work/bin/quickget"

run_create() {
    local root="$1" mode="$2" rc=0
    env PATH="$work/bin" HOME="$work/home" XDG_CACHE_HOME="$work/cache" \
        RYOVM_HOME="$root" TEST_MODE="$mode" bash "$engine" create testos latest desktop >"$work/out" || rc=$?
    if [ "$mode" = success ]; then
        [ "$rc" -eq 0 ]
        jq -se 'last.phase == "done"' "$work/out" >/dev/null
    else
        [ "$rc" -eq 1 ]
        jq -se 'last.phase == "error"' "$work/out" >/dev/null
        if [ "$mode" != conf ]; then
            jq -se 'last.message | contains("URL could not be resolved")' "$work/out" >/dev/null
        fi
    fi
}

root="$work/retry"
run_create "$root" empty
[ ! -e "$root/testos-latest-desktop" ]
run_create "$root" empty
[ ! -e "$root/testos-latest-desktop" ]
printf 'PASS failed downloads can be retried and keep their error\n'

root="$work/existing"
mkdir -p "$root/testos-latest-desktop"
printf 'keep\n' >"$root/testos-latest-desktop/owned"
run_create "$root" empty
[ -f "$root/testos-latest-desktop/owned" ]
printf 'PASS pre-existing directories survive\n'

root="$work/disk"
run_create "$root" disk
[ -f "$root/testos-latest-desktop/disk.qcow2" ]
printf 'PASS disk images survive\n'

root="$work/conf"
run_create "$root" conf
[ ! -e "$root/testos-latest-desktop" ]
[ ! -e "$root/testos-latest-desktop.conf" ]
printf 'PASS failed media with a new config is cleaned\n'

root="$work/conf-existing"
mkdir -p "$root/testos-latest-desktop"
printf 'keep\n' >"$root/testos-latest-desktop/owned"
run_create "$root" conf
[ -f "$root/testos-latest-desktop/owned" ]
printf 'PASS pre-existing directories survive config cleanup\n'

root="$work/success"
run_create "$root" success
[ -f "$root/testos-latest-desktop/install.iso" ]
[ -f "$root/testos-latest-desktop.conf" ]
printf 'PASS successful downloads stay intact\n'

root="$work/symlink"
mkdir -p "$root" "$work/keep"
printf 'keep\n' >"$work/keep/owned"
ln -s "$work/keep" "$root/testos-latest-desktop"
run_create "$root" empty
[ -L "$root/testos-latest-desktop" ]
[ -f "$work/keep/owned" ]
printf 'PASS pre-existing directory links survive\n'
