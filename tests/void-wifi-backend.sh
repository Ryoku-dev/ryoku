#!/usr/bin/env bash
# Hermetic contract checks for the Wi-Fi backend on a runit host. NetworkManager
# D-Bus-activates iwd only where an activation file exists; Void's iwd ships
# none, so the backend switch, the installer and the live medium must keep iwd
# supervised themselves. Never touches the host's services.
set -euo pipefail
ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
helper="$ROOT/system/hardware/network/ryoku-wifi-backend"
[[ -x $helper ]] || { echo "missing executable helper: $helper" >&2; exit 1; }
service_manifest="$ROOT/system/services/manifest.tsv"
void_deploy="$ROOT/void/iso/backend/lib/deploy.sh"
live_setup="$ROOT/void/iso/post-setup.sh"
live_conf="$ROOT/void/iso/rootfs/etc/NetworkManager/conf.d/wifi-backend.conf"
marker=/var/lib/ryoku/wifi-iwd.enabled

fail() { echo "void-wifi-backend: $*" >&2; exit 1; }

# One marker, three writers: the manifest row that links the service, the
# installer that records the pinned backend, and the switch that maintains it.
awk -F '\t' -v cond="condition:$marker" '
  $1 == "wifi-iwd" && $2 == "system" && $3 == "conditional" && $4 == "-" && $5 == "iwd" && $6 == cond { found = 1 }
  END { exit !found }
' "$service_manifest" || fail "manifest lacks the conditional wifi-iwd row keyed on $marker"
grep -qF "write_file /mnt$marker" "$void_deploy" || fail "the Void installer does not record $marker"
grep -qF "$marker" "$helper" || fail "ryoku-wifi-backend does not maintain $marker"
# The live medium pins iwd and must supervise it: no activation file can.
grep -qE '^wifi\.backend=iwd$' "$live_conf" || fail "the live medium does not pin iwd"
grep -qE '^for service in .*\biwd\b' "$live_setup" || fail "post-setup.sh does not link iwd on the live medium"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/systemd"
# Stopping iwd deletes the interfaces it made the way the kernel does, after
# the stop returns and one radio at a time: phy1 at once, phy2 half a second
# later. phy0's interface is not iwd's and stays; phy3 had none to begin with.
cat >"$work/bin/ryoku-host" <<'EOF'
#!/usr/bin/env bash
printf 'ryoku-host %s\n' "$*" >>"$RYOKU_TEST_LOG"
if [[ $* == "svc --system stop iwd" ]]; then
  rm -rf "$RYOKU_TEST_PHYS/phy1/device/net/wlan1"
  (sleep 0.5; rm -rf "$RYOKU_TEST_PHYS/phy2/device/net/wlan2") &
fi
EOF
chmod +x "$work/bin/ryoku-host"
cat >"$work/bin/iw" <<'EOF'
#!/usr/bin/env bash
printf 'iw %s\n' "$*" >>"$RYOKU_TEST_LOG"
EOF
chmod +x "$work/bin/iw"
reset_phys() {
  rm -rf "$work/phys"
  mkdir -p "$work/phys/phy0/device/net/wlan0" "$work/phys/phy1/device/net/wlan1" \
    "$work/phys/phy2/device/net/wlan2" "$work/phys/phy3/device/net"
}
reset_phys

run_helper() {
  : >"$work/log"
  PATH="$work/bin:$PATH" \
  RYOKU_WIFI_BACKEND_CONF="$work/conf.d/wifi-backend.conf" \
  RYOKU_WIFI_IWD_STATE="$work/state/wifi-iwd.enabled" \
  RYOKU_WIFI_BACKEND_HOST="$work/bin/ryoku-host" \
  RYOKU_WIFI_BACKEND_SYSTEMD_DIR="$1" \
  RYOKU_WIFI_BACKEND_PHYS="$work/phys" \
  RYOKU_WIFI_BACKEND_IW="$work/bin/iw" \
  RYOKU_TEST_LOG="$work/log" \
  RYOKU_TEST_PHYS="$work/phys" \
    "$helper" "$2"
}

logged() { grep -qxF "$1" "$work/log"; }

# runit: switching to wpa_supplicant drops the marker and stops iwd; the reconcile
# is what unlinks the service.
run_helper "$work/absent" wpa_supplicant
[[ ! -e $work/state/wifi-iwd.enabled ]] || fail "marker survives a switch to wpa_supplicant"
logged "ryoku-host svc reconcile --system" || fail "no reconcile after switching to wpa_supplicant"
logged "ryoku-host svc --system stop iwd" || fail "iwd not stopped when leaving it"
logged "ryoku-host svc --system restart NetworkManager" || fail "NetworkManager not restarted"
! logged "ryoku-host svc --system start iwd" || fail "iwd started while switching away from it"
# leaving iwd restores every radio iwd left bare, the late one too, and only those
logged "iw phy phy1 interface add wlan1 type managed" || fail "no interface restored for a bare radio after leaving iwd"
logged "iw phy phy2 interface add wlan2 type managed" || fail "a radio iwd left bare late was not restored"
! logged "iw phy phy0 interface add wlan0 type managed" || fail "interface added to a radio that still has one"
! logged "iw phy phy3 interface add wlan3 type managed" || fail "a radio that had no interface before was touched"
[[ $(run_helper "$work/absent" get) == wpa_supplicant ]] || fail "get does not read the pinned wpa_supplicant"

# runit: switching back to iwd records the marker, reconciles, and raises iwd.
run_helper "$work/absent" iwd
[[ -e $work/state/wifi-iwd.enabled ]] || fail "marker missing after switching to iwd"
logged "ryoku-host svc reconcile --system" || fail "no reconcile after switching to iwd"
logged "ryoku-host svc --system stop wpa_supplicant" || fail "wpa_supplicant not stopped when leaving it"
logged "ryoku-host svc --system start iwd" || fail "iwd not started under runit"
! grep -q '^iw ' "$work/log" || fail "interfaces touched while switching to iwd"
[[ $(run_helper "$work/absent" get) == iwd ]] || fail "get does not read the pinned iwd"

# systemd: NetworkManager activates iwd itself; no marker, no reconcile, no start.
rm -rf "$work/state"
run_helper "$work/systemd" iwd
[[ ! -e $work/state/wifi-iwd.enabled ]] || fail "marker written on a systemd host"
! logged "ryoku-host svc reconcile --system" || fail "reconcile run on a systemd host"
! logged "ryoku-host svc --system start iwd" || fail "iwd started by hand on a systemd host"
logged "ryoku-host svc --system stop wpa_supplicant" || fail "wpa_supplicant not stopped on systemd"
logged "ryoku-host svc --system restart NetworkManager" || fail "NetworkManager not restarted on systemd"

echo "void-wifi-backend: ok"
