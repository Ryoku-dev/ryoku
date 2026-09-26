#!/usr/bin/env bash
# container install smoke test: prove a PACKAGED Ryoku install delivers the whole
# ~/.config a user needs. builds the [ryoku] packages from THIS checkout into a
# local repo, installs ryoku-desktop (which pulls every depend), materializes the
# config as a throwaway user, then asserts the materialized tree. this catches
# the "config file lands in the repo but no package ships it" class the delivery
# contract exists to prevent (docs/updates.md).
#
# runs as root inside an Arch or CachyOS container (archlinux:latest in CI).
# heavy (full build + desktop install), so it is driven by the install-test
# workflow (dispatch/schedule), never on every push.
#
#   installation/tests/container-install.sh [arch|cachyos]   (also RYOKU_TEST_BASE, default arch)
set -euo pipefail

BASE=${1:-${RYOKU_TEST_BASE:-arch}}
REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
ARCH=x86_64
OUT="$REPO/release/repo/out/$ARCH"
TESTUSER=ryokutest

log() { printf '\033[1;35m::\033[0m %s\n' "$*"; }
die() { printf 'container-install: error: %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must run as root (installs packages); got uid $EUID"

# CachyOS differs only in its extra signing keyring; everything downstream (build,
# materialize, assertions) is identical.
case "$BASE" in
  arch)    keyring=(archlinux-keyring) ;;
  cachyos) keyring=(cachyos-keyring archlinux-keyring) ;;
  *)       die "unknown base '$BASE' (want arch|cachyos)" ;;
esac
log "base: $BASE"

# 1. host toolchain. refresh the keyring first so signature checks on the freshly
#    synced core/extra pass on a stale base image, then the build set from
#    release/repo/build-toolchain.packages -- the same file publish-repo.yml
#    reads. makepkg's deps live on the host because build-repo.sh builds
#    --nodeps.
#
#    That file exists because this list used to be a second hand-maintained copy
#    with a comment claiming it matched publish-repo.yml. It drifted: `meson`
#    landed in the other copy for the controller packages, this gate went red on
#    the first main sync, and the publish job was skipped as a result.
pacman -Sy --noconfirm --needed "${keyring[@]}"
if [[ ${RYOKU_PREBUILT_REPO:-0} == 1 ]]; then
  # nothing to build: only what the assertions below call
  pacman -Syu --noconfirm --needed jq gnupg libarchive
else
  mapfile -t toolchain < <(grep -vE '^[[:space:]]*(#|$)' "$REPO/release/repo/build-toolchain.packages")
  pacman -Syu --noconfirm --needed "${toolchain[@]}"
fi

# pacman 7 runs install scriptlets in a sandbox that cannot open a network
# namespace inside a container, so post-install hooks (fc-cache, icon cache, ...)
# error out. they are irrelevant here; disable the sandbox for this run.
grep -q '^DisableSandboxNetwork' /etc/pacman.conf \
  || sed -i '/^\[options\]/a DisableSandboxNetwork' /etc/pacman.conf

# 2. the [ryoku] packages. RYOKU_PREBUILT_REPO=1 means publish-repo.yml already
#    built and signed them with the release key and dropped the repo at $OUT
#    (the bytes it will upload), so install those and verify their signatures
#    the way a user's pacman does. otherwise build them here into a local repo
#    with a throwaway key, consumed with SigLevel=Never below (shared with the
#    VM install test).
if [[ ${RYOKU_PREBUILT_REPO:-0} == 1 ]]; then
  [[ -f "$OUT/ryoku.db" ]] || die "RYOKU_PREBUILT_REPO=1 but no repo at $OUT"
  log "using the prebuilt signed repo at $OUT ($(find "$OUT" -name '*.pkg.tar.zst' | wc -l) packages)"
  repo_name=ryoku
  siglevel=Required
  pacman-key --init >/dev/null 2>&1 || true
  pacman-key --add "$REPO/release/packages/ryoku-keyring/ryoku.gpg"
  keyid=$(gpg --homedir /etc/pacman.d/gnupg --with-colons --show-keys "$REPO/release/packages/ryoku-keyring/ryoku.gpg" 2>/dev/null | awk -F: '/^fpr/{print $10; exit}')
  pacman-key --lsign-key "$keyid" >/dev/null
else
  log "building [ryoku] packages from the checkout -> $OUT"
  RYOKU_REPO_NAME=ryoku-local "$REPO/installation/tests/build-ryoku-repo.sh"
  repo_name=ryoku-local
  siglevel=Never
fi

# Regression guard for the CachyOS oh-my-zsh-git swap: cachyos-zsh-config depends
# on oh-my-zsh-git, so ryoku-oh-my-zsh must claim it in provides + conflict +
# replaces or `pacman -Syu ryoku-desktop` deadlocks on that box (see the package
# PKGBUILD). Assert it on the freshly built package before we ever publish.
omz=$(find "$OUT" -name 'ryoku-oh-my-zsh-*.pkg.tar.zst' | head -1)
[[ -n $omz ]] || die "ryoku-oh-my-zsh package was not built"
pkginfo=$(bsdtar -xOf "$omz" .PKGINFO)
for field in provides conflict replaces; do
  grep -qE "^$field = oh-my-zsh-git" <<<"$pkginfo" \
    || die "ryoku-oh-my-zsh is missing '$field = oh-my-zsh-git'; a CachyOS box (cachyos-zsh-config) would deadlock the install"
done
log "oh-my-zsh-git swap metadata present on ryoku-oh-my-zsh"

# 3. register the repo and install the desktop. the prebuilt repo is verified
#    against the release key (SigLevel=Required, as on a user's box); a local
#    throwaway-key build relaxes verification for THIS repo only. official
#    repos keep their SigLevel. an unresolved depend must fail loudly here --
#    that is the whole point.
cat >>/etc/pacman.conf <<EOF

[$repo_name]
SigLevel = $siglevel
Server = file://$OUT
EOF

pacman -Sy --noconfirm
log "installing ryoku-desktop from [$repo_name]"
pacman -S --noconfirm ryoku-desktop

[[ -d /usr/share/ryoku/config ]] || die "ryoku-desktop did not lay /usr/share/ryoku/config"
# the compositor split: the base pulls its sole variant through the
# ryoku-desktop-compositor virtual, which ships the hypr tree and the provider.
pacman -Qq ryoku-desktop-hyprland >/dev/null 2>&1 \
  || die "ryoku-desktop did not pull ryoku-desktop-hyprland (compositor-split auto-pull broken)"
[[ -x /usr/bin/ryoku-wm-hyprland ]] || die "ryoku-desktop-hyprland did not ship the ryoku-wm-hyprland provider"
[[ -x /usr/bin/ryoku ]] || die "the ryoku CLI was not installed"
# the config bootstrap: a package install lays nothing into ~/.config, so the
# first login after one boots a bare compositor unless this unit runs first.
[[ -f /usr/lib/systemd/user/ryoku-bootstrap.service ]] || die "ryoku-desktop did not ship the config bootstrap unit"

# 4. materialize as a throwaway user, forcing HOME/USER like deploy.sh's
#    ryoku_deploy_materialize (runuser keeps root's env otherwise).
id "$TESTUSER" &>/dev/null || useradd --create-home "$TESTUSER"
log "materializing config as $TESTUSER"
runuser -u "$TESTUSER" -- env \
  "HOME=/home/$TESTUSER" "USER=$TESTUSER" "LOGNAME=$TESTUSER" \
  ryoku materialize

# 5. assert the materialized ~/.config is complete: a representative, high-signal
#    slice spanning shell, compositor, palette, and every per-app config.
cfg="/home/$TESTUSER/.config"
files=(
  quickshell/shell/shell.qml
  hypr/hyprland.lua
  fish/config.fish
  ryoku-terminal/env.sh
  bash/ryoku.bash
  bash/rashin.bash
  zsh/ryoku.zsh
  zsh/rashin.zsh
  starship.toml
  kitty/kitty.conf
  yazi/yazi.toml
  nvim/init.lua
  pip/pip.conf
  wireplumber/wireplumber.conf.d/51-ryoku-bluetooth.conf
  chromium-flags.conf
  chrome-flags.conf
)
dirs=(quickshell/hub)

missing=()
for f in "${files[@]}"; do
  [[ -s "$cfg/$f" ]] || missing+=("$cfg/$f")
done
for d in "${dirs[@]}"; do
  [[ -n $(find "$cfg/$d" -mindepth 1 -type f -print -quit 2>/dev/null) ]] \
    || missing+=("$cfg/$d/ (empty or absent)")
done
# the nvim handler and the default-app map ship system-wide (not materialized:
# ~/.config/mimeapps.list belongs to the user's own "Set as default" picks), so
# check the system tree.
[[ -s /usr/share/applications/ryoku-nvim.desktop ]] \
  || missing+=("/usr/share/applications/ryoku-nvim.desktop")
[[ -s /usr/share/applications/mimeapps.list ]] \
  || missing+=("/usr/share/applications/mimeapps.list")
if [[ -e "$cfg/mimeapps.list" ]]; then
  missing+=("$cfg/mimeapps.list (materialize must not create the user's default-app file)")
fi

# Every Ryoku.* QML module the shell (and hub/apps) imports must be installed on
# the system Qt import path. deploy.sh puts them there for a dev box, so a package
# that forgets one ships a desktop only a dev box can run: qs -c shell fails the
# import at login and paints a bare grey Hyprland screen with no shell. Derive the
# set from the source imports so a new module cannot be added without packaging it.
mapfile -t ryoku_mods < <(
  grep -rhoE "import Ryoku\.[A-Za-z0-9]+" \
    "$REPO/ryoku/shell" "$REPO/ryoku/hub" "$REPO/ryoku/apps" "$REPO/ryoku/ui" 2>/dev/null \
    | awk '{print $2}' | sort -u
)
for mod in "${ryoku_mods[@]}"; do
  [[ -s "/usr/lib/qt6/qml/Ryoku/${mod#Ryoku.}/qmldir" ]] \
    || missing+=("/usr/lib/qt6/qml/Ryoku/${mod#Ryoku.}/qmldir (imported as $mod, but no package installs it)")
done

if (( ${#missing[@]} )); then
  echo "container-install: FAIL -- packaged install is missing config a user needs:" >&2
  printf '  - %s\n' "${missing[@]}" >&2
  exit 1
fi

# 6. every shipped QML root must LOAD against the installed Qt modules, not just
#    exist. One file that cannot instantiate (a handler on a signal the type does
#    not have, a property that is not there) blanks its whole surface: a Hub page
#    renders empty, a shell root paints bare Hyprland. This is the same import
#    path a user's session resolves, so a finding here is a finding on their box.
log "linting the materialized QML for load failures"
"$REPO/bin/ryoku-dev-lint-qml" "$cfg/quickshell/shell" "$cfg/quickshell/hub" \
  || die "shipped QML cannot load (see above); a user's desktop would come up broken"

# 7. release naming and channels. the package must carry the /etc/ryoku-release
#    marker (a hand build names itself local-<pkgver>), the CLI must read it
#    and report the channel from the [ryoku] Server line, and `ryoku track`
#    must refuse to move a box whose [ryoku] points at a mirror Ryoku does not
#    publish (this local repo), instead of rewriting it into a channel it
#    cannot serve. the live switch itself (stable -> testing -> a pinned
#    release) needs the published channels and runs in the VM install test.
log "checking release naming and channel handling"
[[ -f /etc/ryoku-release ]] || die "ryoku-desktop did not ship /etc/ryoku-release"
# a prebuilt repo carries the release and channel the publish named; a hand
# build names itself local-<pkgver> on channel local.
expect_release=$(jq -r '.release // empty' "$OUT/release.json" 2>/dev/null || true)
expect_channel=$(jq -r '.channel // empty' "$OUT/release.json" 2>/dev/null || true)
: "${expect_release:=local-}" "${expect_channel:=local}"
grep -qF "RELEASE=$expect_release" /etc/ryoku-release || die "unexpected /etc/ryoku-release (want RELEASE=$expect_release*): $(cat /etc/ryoku-release)"
grep -qx "CHANNEL=$expect_channel" /etc/ryoku-release || die "expected CHANNEL=$expect_channel in /etc/ryoku-release"
name=$(tr -d '[:space:]' < "$REPO/CODENAME")
grep -qx "NAME=$name" /etc/ryoku-release || die "ryoku-desktop did not carry the line's name ($name) into /etc/ryoku-release"
ver=$(runuser -u "$TESTUSER" -- env "HOME=/home/$TESTUSER" ryoku version)
[[ $ver == "$expect_release"* ]] || die "ryoku version should print the release marker ($expect_release*), got: $ver"
pretty=$(runuser -u "$TESTUSER" -- env "HOME=/home/$TESTUSER" ryoku version --pretty)
[[ $pretty == "$name $expect_release"* ]] || die "ryoku version --pretty should lead with the name, got: $pretty"

# the control manifest must be published beside release.json and be the real
# thing: every lane present, the first-party set non-empty, and the release it
# names the one the channel serves. A channel that serves no manifest makes
# `ryoku update`'s convergence a no-op on every box, which is exactly the
# "works here, inert for users" failure this gate exists to catch.
mf="$OUT/manifest.json"
[[ -s "$mf" ]] || die "the published repo carries no manifest.json"
jq -e '.schema == 1 and (.base|length > 0) and (.firstParty|length > 0) and (.provisioned|length > 0)' "$mf" >/dev/null \
  || die "manifest.json is malformed or missing lanes: $(jq -c '{schema,base:(.base|length),firstParty:(.firstParty|length),provisioned:(.provisioned|length)}' "$mf" 2>&1)"
mf_release=$(jq -r '.release' "$mf")
[[ $mf_release == "$expect_release"* ]] \
  || die "manifest.json names release $mf_release, not the one release.json serves ($expect_release*)"
if [[ $repo_name != ryoku ]]; then
  cat >>/etc/pacman.conf <<EOF

[ryoku]
SigLevel = Never
Server = file://$OUT
EOF
fi
if runuser -u "$TESTUSER" -- env "HOME=/home/$TESTUSER" ryoku track testing 2>/tmp/track.err; then
  die "ryoku track must refuse a [ryoku] repo Ryoku does not publish"
fi
grep -q "does not publish" /tmp/track.err || die "unexpected track refusal: $(cat /tmp/track.err)"
grep -q "^Server = file://$OUT" /etc/pacman.conf || die "track rewrote a foreign [ryoku] Server line"
if [[ $repo_name != ryoku ]]; then
  # The probe's section must not outlive it: an unsynced [ryoku] (a local
  # build's db is ryoku-local.db, so this section can never sync) makes every
  # later pacman transaction abort with "could not find database", hiding real
  # failures behind a test artifact.
  sed -i "\|^\[ryoku\]$|,\|^Server = file://$OUT$|d" /etc/pacman.conf
  grep -q "^\[ryoku\]$" /etc/pacman.conf && die "the ephemeral [ryoku] section survived its probe"
fi

# 8. the boot guard. the ryoku package ships the unit and the tmpfiles entry
#    for the sessions' boot records; `ryoku boot-guard` with nothing pending is
#    a no-op, and a marker whose desktop was proven up in another boot is
#    disarmed rather than counted.
log "checking the boot guard"
[[ -f /usr/lib/systemd/system/ryoku-boot-guard.service ]] || die "ryoku did not ship ryoku-boot-guard.service"
[[ -f /usr/lib/tmpfiles.d/ryoku.conf ]] || die "ryoku did not ship its tmpfiles entry"
systemd-tmpfiles --create /usr/lib/tmpfiles.d/ryoku.conf
[[ -d /var/lib/ryoku/boot ]] || die "tmpfiles did not create /var/lib/ryoku/boot"
[[ $(stat -c %a /var/lib/ryoku/boot) == 1777 ]] || die "/var/lib/ryoku/boot must be 1777, got $(stat -c %a /var/lib/ryoku/boot)"
ryoku boot-guard || die "boot-guard with nothing pending must succeed"
printf '{"from":"v0.0.1","to":"v0.0.2","armedBoot":"armed-boot","boots":0,"at":"x"}\n' >/var/lib/ryoku/update-pending.json
runuser -u "$TESTUSER" -- sh -c "echo later-boot >/var/lib/ryoku/boot/ok-$(id -u "$TESTUSER")" || die "a session cannot record its boot"
ryoku boot-guard | grep -q "disarmed" || die "a proven boot must disarm the guard"
[[ ! -e /var/lib/ryoku/update-pending.json ]] || die "disarm left the marker behind"

# 9. the lid/sleep policy. logind is the safe fallback outside a Ryoku desktop
#    session; the session-long clamshell inhibitor takes ownership while the
#    shell can guarantee lock-before-suspend. Assert that the package owns the
#    policy and both transaction hooks that adopt it in every live graphical
#    user. `systemd-analyze cat-config` is the offline merge logind itself
#    performs, so a drop-in the checkout carries but no package installs (or one
#    placed where logind never scans) shows up missing here.
log "checking the lid/sleep policy drop-in"
lid_conf=/etc/systemd/logind.conf.d/10-ryoku-lid.conf
[[ -f $lid_conf ]] || die "ryoku-desktop did not ship $lid_conf"
owner=$(pacman -Qo "$lid_conf" 2>&1 | tail -1) \
  || die "$lid_conf is not owned by an installed package"
# Drain pacman's list into a variable before matching: `pacman -Ql pkg | grep -q`
# under pipefail is a flake, because grep exits at the first match, SIGPIPEs
# pacman mid-write, and the pipeline reports failure on a package that does
# claim the file.
desktop_files=$(pacman -Ql ryoku-desktop) \
  || die "pacman -Ql ryoku-desktop failed"
[[ $desktop_files == *logind.conf.d/10-ryoku-lid.conf* ]] \
  || die "ryoku-desktop does not claim $lid_conf (owner: $owner)"
lid_cat=$(systemd-analyze cat-config systemd/logind.conf) \
  || die "systemd-analyze cat-config systemd/logind.conf failed in this container"
grep -qxF "# $lid_conf" <<<"$lid_cat" \
  || die "logind never reads $lid_conf (absent from cat-config); the lid policy is inert"
for setting in \
  HandleLidSwitch=suspend \
  HandleLidSwitchExternalPower=suspend \
  HandleLidSwitchDocked=ignore \
  InhibitDelayMaxSec=15; do
  grep -qxF "$setting" <<<"$lid_cat" \
    || die "$lid_conf is not effective: cat-config systemd/logind.conf lacks '$setting'"
done
for artifact in \
  /usr/bin/ryoku-power-cutover \
  /usr/share/libalpm/hooks/94-ryoku-power-cutover-prepare.hook \
  /usr/share/libalpm/hooks/95-ryoku-power-cutover.hook; do
  [[ -e $artifact ]] || die "ryoku-desktop did not ship $artifact"
  pacman -Qo "$artifact" >/dev/null 2>&1 \
    || die "$artifact is not owned by an installed package"
done
[[ -x /usr/bin/ryoku-power-cutover ]] \
  || die "the packaged power cutover helper is not executable"
grep -qxF 'Exec = /usr/bin/ryoku-power-cutover prepare-package' \
  /usr/share/libalpm/hooks/94-ryoku-power-cutover-prepare.hook \
  || die "the packaged pre-transaction hook does not preserve its live-session executor"
# The post hook execs the installed binary, which prefers the staged /run copy
# when the pre-transaction hook ran and falls back to itself on the transaction
# that first installs the pair (#272): asserting the /run path here would pin
# the very bug that made first installs report a failed hook.
grep -qxF 'Exec = /usr/bin/ryoku-power-cutover package' \
  /usr/share/libalpm/hooks/95-ryoku-power-cutover.hook \
  || die "the packaged post-transaction hook does not run the guarded adoption"
grep -q 'RYOKU_CUTOVER_STAGED' /usr/bin/ryoku-power-cutover \
  || die "the packaged cutover helper does not prefer the staged pre-upgrade copy"

# 10. the other compositor variant. The testing channel ships one variant package
#    per window manager and the base pulls whichever the virtual resolves to, so
#    a hyprland-only install test says nothing about niri: a broken niri package
#    would publish green. Installing it exercises the second half the switch
#    offers, and the provider's own output is validated with niri's parser -- the
#    niri twin of the Hyprland assertions above.
log "installing the second compositor variant"
# The variants coexist: installing the target leaves the outgoing desktop in
# place, which is what makes a switch back instant and the keep-or-remove choice
# honest. Only `ryoku wm use --remove-previous` takes one out.
pacman -S --needed --noconfirm ryoku-desktop-niri || die "ryoku-desktop-niri did not install beside ryoku-desktop-hyprland"
pacman -Qq ryoku-desktop-niri >/dev/null 2>&1 || die "the niri variant is not installed"
pacman -Qq ryoku-desktop-hyprland >/dev/null 2>&1 || die "installing the niri variant removed the hyprland one (variants must coexist)"
[[ -x /usr/bin/ryoku-wm-niri ]] || die "ryoku-desktop-niri did not ship the ryoku-wm-niri provider"
[[ -f /usr/share/ryoku/config/niri/config.kdl ]] || die "ryoku-desktop-niri did not ship the niri config tree"
pacman -Ql ryoku-desktop-niri | grep -q "config/hypr" && die "the niri variant ships the Hyprland tree"
[[ -f /usr/share/wayland-sessions/niri.desktop ]] || die "the niri session entry is not installed"
# niri is a systemd user unit, so it can order the bootstrap ahead of itself: the
# drop-in is what makes the *first* login after an install a Ryoku desktop.
[[ -f /usr/lib/systemd/user/niri.service.d/ryoku-bootstrap.conf ]] || die "the niri variant did not ship the bootstrap ordering drop-in"

# materialize for the test user, then let the packaged provider author the
# generated includes (settings.kdl, rebinds.kdl) the way a login does, and let
# niri parse the result: an update that ships a broken tree must fail here.
log "materializing the niri config and validating it with niri"
runuser -u "$TESTUSER" -- env "HOME=/home/$TESTUSER" "USER=$TESTUSER" "LOGNAME=$TESTUSER" \
  ryoku materialize >/dev/null || die "ryoku materialize failed on the niri variant"
[[ -f "$cfg/niri/config.kdl" ]] || die "materialize did not lay the niri config"
mkdir -p "$cfg/ryoku"
printf '{"desktop":{},"wm":{"niri":{}}}\n' >"$cfg/ryoku/desktop.json"
runuser -u "$TESTUSER" -- env "HOME=/home/$TESTUSER" "XDG_CURRENT_DESKTOP=niri" \
  /usr/bin/ryoku-wm-niri apply "$cfg/ryoku/desktop.json" >/dev/null \
  || die "the niri provider did not apply the store"
[[ -f "$cfg/niri/settings.kdl" && -f "$cfg/niri/rebinds.kdl" ]] || die "the niri provider did not write its generated includes"
niri validate -c "$cfg/niri/config.kdl" || die "the niri config the packages ship does not parse"

log "container-install: OK -- ryoku-desktop delivered the full config to $cfg"
