#!/usr/bin/env bash
# hermetic test for ryoku/shell/scripts/stash-install.sh. stash makes dropped
# files launchable: AppImages + self-contained tarballs get a synth desktop
# entry; an Arch package (.pkg.tar.zst, recognised by its .PKGINFO member) must
# go through the host package seam via pkexec, never be extracted into ~/.local
# where its /opt binary would not work. The privileged commands are stubbed so
# the test does not touch the real system.
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SCRIPT="$ROOT/ryoku/shell/scripts/stash-install.sh"
[[ -f $SCRIPT ]] || { echo "::error::missing $SCRIPT" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fail=0
check() { if [[ $1 == "$2" ]]; then echo "  ok: $3"; else echo "::error::FAIL: $3 (got '$1' want '$2')" >&2; fail=1; fi; }
present() { if [[ -e $1 ]]; then echo "  ok: $2"; else echo "::error::FAIL: $2 (missing $1)" >&2; fail=1; fi; }
absent()  { if [[ ! -e $1 ]]; then echo "  ok: $2"; else echo "::error::FAIL: $2 ($1 exists)" >&2; fail=1; fi; }

# stubs. pkexec records the escalation and runs the ryoku-host stub so its exit
# status matches the host seam. notify-send and update-desktop-database only
# need to exist for the script's presence checks.
stub="$work/bin"
mkdir -p "$stub"
cat >"$stub/pkexec" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$work/pkexec.log"
exec "\$@"
EOF
cat >"$stub/ryoku-host" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$work/ryoku-host.log"
exit "\${RYOKU_TEST_INSTALL_FILE_RC:-0}"
EOF
cat >"$stub/flatpak" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$work/flatpak.log"
exit 0
EOF
for t in notify-send update-desktop-database; do printf '#!/bin/sh\nexit 0\n' >"$stub/$t"; done
chmod +x "$stub"/*
export PATH="$stub:$PATH"

# fake Arch package: a tar with a top-level .PKGINFO (pkgname=$2). gz so no zstd
# needed; classify keys on the .pkg.tar. infix or the .PKGINFO member.
mkpkg() {
  local out="$1" name="$2" d="$work/pkgsrc"
  rm -rf "$d"; mkdir -p "$d"
  printf 'pkgname = %s\npkgver = 1-1\n' "$name" >"$d/.PKGINFO"
  ( cd "$d" && tar -czf "$out" .PKGINFO )
}

run_stash() {
  rm -rf "${work:?}/home"
  HOME="$work/home" bash "$SCRIPT" >"$work/out" 2>&1
}

# case 1: .pkg.tar.* installs via pkexec and the host seam, not extraction.
s1="$work/stash1"; mkdir -p "$s1"
mkpkg "$s1/foo.pkg.tar.gz" foo
: >"$work/pkexec.log"
: >"$work/ryoku-host.log"
STASH_DIR="$s1" run_stash
if grep -q "Installed foo" "$work/out"; then
  echo "  ok: reported the pkgname (foo)"
else
  echo "::error::FAIL: did not report pkgname; out: $(cat "$work/out")" >&2
  fail=1
fi
check "$(cat "$work/pkexec.log")" "ryoku-host pkg install-file $s1/foo.pkg.tar.gz" \
  "Arch package handed to the host package seam through pkexec"
check "$(cat "$work/ryoku-host.log")" "pkg install-file $s1/foo.pkg.tar.gz" \
  "host package seam receives the archive"
absent "$work/home/.local/share/ryoku-apps/foo" "Arch package NOT extracted into ~/.local"
absent "$s1/foo.pkg.tar.gz" "source removed from the stash after a successful install"
if grep -qx "@AUTH" "$work/out"; then
  echo "  ok: privileged package install signals the deck to step aside (@AUTH)"
else
  echo "::error::FAIL: privileged package install did not emit @AUTH" >&2; fail=1
fi

# case 2: renamed package (.tar.gz with .PKGINFO) still detected as an Arch package.
s2="$work/stash2"; mkdir -p "$s2"
mkpkg "$s2/bar.tar.gz" bar
: >"$work/pkexec.log"
STASH_DIR="$s2" run_stash
check "$(cat "$work/pkexec.log")" "ryoku-host pkg install-file $s2/bar.tar.gz" \
  "renamed Arch package detected by .PKGINFO and sent through the host seam"

# case 2b: a host without install-file refuses Arch archives and keeps them.
s2b="$work/stash2b"; mkdir -p "$s2b"
mkpkg "$s2b/void.pkg.tar.gz" void
: >"$work/pkexec.log"
RYOKU_TEST_INSTALL_FILE_RC=5 STASH_DIR="$s2b" run_stash || true
if grep -qx "Arch packages cannot be installed on this system." "$work/out"; then
  echo "  ok: unsupported Arch package has a clear host-specific error"
else
  echo "::error::FAIL: missing unsupported Arch package message; out: $(cat "$work/out")" >&2
  fail=1
fi
if grep -qi "pacman" "$work/out"; then
  echo "::error::FAIL: unsupported host output recommends pacman" >&2
  fail=1
else
  echo "  ok: unsupported host output does not recommend pacman"
fi
present "$s2b/void.pkg.tar.gz" "unsupported Arch package remains in the stash"

# case 3: generic tarball (no .PKGINFO) stays on the extraction path.
s3="$work/stash3"; mkdir -p "$s3"
g="$work/gen"; rm -rf "$g"; mkdir -p "$g"
printf '#!/bin/sh\necho hi\n' >"$g/plainbin"; chmod +x "$g/plainbin"
( cd "$g" && tar -czf "$s3/plainapp.tar.gz" plainbin )
: >"$work/pkexec.log"
STASH_DIR="$s3" run_stash
check "$(cat "$work/pkexec.log")" "" "generic tarball never escalated to the host package installer"
present "$work/home/.local/share/applications/plainapp.desktop" \
  "generic tarball still becomes a launcher entry"
if grep -qx "@AUTH" "$work/out"; then
  echo "::error::FAIL: a non-privileged install emitted @AUTH" >&2; fail=1
else
  echo "  ok: non-privileged install does not signal the deck"
fi

# case 4: flatpak bundle -> flatpak install --user (flatpak stubbed).
s4="$work/stash4"; mkdir -p "$s4"
: >"$s4/app.flatpak"
: >"$work/pkexec.log"; : >"$work/flatpak.log"
STASH_DIR="$s4" run_stash
if grep -qx "install --user --noninteractive $s4/app.flatpak" "$work/flatpak.log"; then
  echo "  ok: flatpak bundle handed to flatpak install --user"
else
  echo "::error::FAIL: flatpak bundle not installed; log: $(cat "$work/flatpak.log")" >&2
  fail=1
fi
check "$(cat "$work/pkexec.log")" "" "flatpak never escalated to pacman"

# case 5: .deb payload extracted into a launcher entry (best-effort). needs ar
# (to build the fixture) + bsdtar (libarchive); skipped if either missing.
if command -v ar >/dev/null 2>&1 && command -v bsdtar >/dev/null 2>&1; then
  s5="$work/stash5"; mkdir -p "$s5"
  dr="$work/debroot"; rm -rf "$dr"; mkdir -p "$dr/usr/bin" "$dr/usr/share/applications"
  printf '#!/bin/sh\necho hi\n' >"$dr/usr/bin/debapp"; chmod +x "$dr/usr/bin/debapp"
  printf '[Desktop Entry]\nType=Application\nName=DebApp\nExec=debapp\n' >"$dr/usr/share/applications/debapp.desktop"
  ( cd "$dr" && tar -czf "$work/data.tar.gz" usr )
  printf '2.0\n' >"$work/debian-binary"
  ( cd "$work" && ar rc "$s5/debapp.deb" debian-binary data.tar.gz )
  : >"$work/pkexec.log"; : >"$work/flatpak.log"
  STASH_DIR="$s5" run_stash
  check "$(cat "$work/pkexec.log")" "" "deb payload never escalated to pacman"
  present "$work/home/.local/share/applications/debapp.desktop" "deb payload extracted into a launcher entry"

  # case 5b: a .deb whose desktop Exec is an absolute /opt path must resolve to
  # that exact binary, never a same-named decoy elsewhere in the payload.
  # Termius ships etc/cron.daily/termius-app -- sorts before opt/, so the old
  # tree-wide basename search grabbed the cron job and the app never launched.
  s5b="$work/stash5b"; mkdir -p "$s5b"
  dr2="$work/debroot2"; rm -rf "$dr2"
  mkdir -p "$dr2/opt/realapp" "$dr2/etc/cron.daily" "$dr2/usr/share/applications"
  printf '#!/bin/sh\necho real\n' >"$dr2/opt/realapp/realapp"; chmod +x "$dr2/opt/realapp/realapp"
  printf '#!/bin/sh\necho cron\n' >"$dr2/etc/cron.daily/realapp"; chmod +x "$dr2/etc/cron.daily/realapp"
  printf '[Desktop Entry]\nType=Application\nName=RealApp\nExec=/opt/realapp/realapp %%U\n' \
    >"$dr2/usr/share/applications/realapp.desktop"
  ( cd "$dr2" && tar -czf "$work/data.tar.gz" opt etc usr )
  printf '2.0\n' >"$work/debian-binary"
  ( cd "$work" && ar rc "$s5b/realapp.deb" debian-binary data.tar.gz )
  : >"$work/pkexec.log"
  STASH_DIR="$s5b" run_stash
  ent="$work/home/.local/share/applications/realapp.desktop"
  if grep -q "^Exec=.*/opt/realapp/realapp" "$ent" 2>/dev/null && ! grep -q "cron.daily" "$ent" 2>/dev/null; then
    echo "  ok: absolute deb Exec resolves to the opt binary, not the cron decoy"
  else
    echo "::error::FAIL: deb Exec mis-resolved: $(grep '^Exec=' "$ent" 2>/dev/null)" >&2; fail=1
  fi
else
  echo "  skip: .deb extraction (ar or bsdtar absent)"
fi

# case 6: RYOKU_STASH_KEEP=1 keeps the source after a successful install.
s6="$work/stash6"; mkdir -p "$s6"
mkpkg "$s6/keep.pkg.tar.gz" keep
RYOKU_STASH_KEEP=1 STASH_DIR="$s6" run_stash
present "$s6/keep.pkg.tar.gz" "RYOKU_STASH_KEEP=1 keeps the source in the stash"

# case 7: a failed install leaves the source in place (no cleanup on failure).
s7="$work/stash7"; mkdir -p "$s7"
printf 'not a real package\n' >"$s7/broken.deb"
STASH_DIR="$s7" run_stash || true
present "$s7/broken.deb" "a failed install keeps its source in the stash"

if (( fail )); then echo "stash-install: FAILED" >&2; exit 1; fi
echo "stash-install: all checks passed"
