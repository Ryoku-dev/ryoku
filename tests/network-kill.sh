#!/usr/bin/env bash
# Hermetic contract checks for the privileged full-network kill switch.
# Never uses the host nftables or NetworkManager binaries.
set -euo pipefail

ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
helper="$ROOT/system/hardware/network/ryoku-network-kill"

[[ -x $helper ]] || { echo "missing executable helper: $helper" >&2; exit 1; }
polkit="$ROOT/system/hardware/network/55-ryoku-network-kill.rules"
guard="$ROOT/system/hardware/network/ryoku-network-kill-guard.service"
disconnect="$ROOT/system/hardware/network/ryoku-network-kill-disconnect.service"
page="$ROOT/ryoku/hub/quickshell/pages/ConnectionsPage.qml"
pkgbuild="$ROOT/release/packages/ryoku-desktop/PKGBUILD"
install_hook="$ROOT/release/packages/ryoku-desktop/ryoku-desktop.install"
base_packages="$ROOT/system/packages/base.packages"

grep -qxF nftables "$base_packages"
grep -qF "  'nftables'" "$pkgbuild"
grep -qF '55-ryoku-network-kill.rules' "$pkgbuild"
grep -qF 'ryoku-network-kill-guard.service' "$pkgbuild"
grep -qF 'RequiredBy=NetworkManager.service' "$guard"
grep -qF 'WantedBy=multi-user.target NetworkManager.service' "$disconnect"
grep -qF 'ryoku-network-kill-disconnect.service' "$pkgbuild"
post_install="$(sed -n '/^post_install()/,/^}/p' "$install_hook")"
post_upgrade="$(sed -n '/^post_upgrade()/,/^}/p' "$install_hook")"
if grep -qF '_network_kill_units' <<<"$post_install"; then
  echo "package install must leave kill-switch units disabled" >&2
  exit 1
fi
grep -qF '_network_kill_units' <<<"$post_upgrade"

grep -qF '["pkexec", "/usr/bin/ryoku-network-kill", "status"]' "$page"
grep -qF 'killSetProc.target = killState === "off" ? "on" : "off";' "$page"
grep -qF 'Blocks Wi-Fi, Ethernet, VPN, LAN and IPv4/IPv6. It also severs SSH.' "$page"

grep -qF '"/usr/bin/ryoku-network-kill"' "$polkit"
grep -qF 'ConditionPathExists=/var/lib/ryoku/network-kill-switch.enabled' "$guard"
grep -qF 'Before=network-pre.target NetworkManager.service' "$guard"
grep -qF 'ExecStart=/usr/bin/nmcli networking off' "$disconnect"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/state"

cat >"$work/bin/nft" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'nft %s\n' "$*" >>"$RYOKU_NETWORK_KILL_LOG"
case "$*" in
  "list table inet ryoku_kill_switch")
    [[ -e $RYOKU_NETWORK_KILL_FAKE_TABLE ]]
    ;;
  "--check -f -")
    cat >"$RYOKU_NETWORK_KILL_CHECKED_RULES"
    ;;
  "-f -")
    cat >"$RYOKU_NETWORK_KILL_APPLIED_RULES"
    : >"$RYOKU_NETWORK_KILL_FAKE_TABLE"
    ;;
  "delete table inet ryoku_kill_switch")
    rm -f "$RYOKU_NETWORK_KILL_FAKE_TABLE"
    ;;
  *)
    echo "unexpected nft command: $*" >&2
    exit 64
    ;;
esac
EOF

cat >"$work/bin/nmcli" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'nmcli %s\n' "$*" >>"$RYOKU_NETWORK_KILL_LOG"
EOF

cat >"$work/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'systemctl %s\n' "$*" >>"$RYOKU_NETWORK_KILL_LOG"
EOF
chmod +x "$work/bin"/*

run() {
  PATH="$work/bin:$PATH" \
  RYOKU_NETWORK_KILL_STATE="$work/state/enabled" \
  RYOKU_NETWORK_KILL_LOG="$work/log" \
  RYOKU_NETWORK_KILL_FAKE_TABLE="$work/table" \
  RYOKU_NETWORK_KILL_CHECKED_RULES="$work/checked.nft" \
  RYOKU_NETWORK_KILL_APPLIED_RULES="$work/applied.nft" \
  "$helper" "$@"
}

line_of() {
  grep -nF "$1" "$work/log" | cut -d: -f1 | command head -n1
}

: >"$work/log"
run on
[[ -e $work/state/enabled ]] || { echo "on did not persist armed state" >&2; exit 1; }
[[ -e $work/table ]] || { echo "on did not install firewall" >&2; exit 1; }
grep -qxF 'systemctl enable --quiet ryoku-network-kill-guard.service ryoku-network-kill-disconnect.service' "$work/log"
grep -qxF 'nmcli networking off' "$work/log"
[[ $(line_of 'nft -f -') -lt $(line_of 'nmcli networking off') ]] || {
  echo "NetworkManager was disabled before firewall was installed" >&2; exit 1;
}
for fragment in \
  'table inet ryoku_kill_switch' \
  'type filter hook input priority -300; policy accept;' \
  'type filter hook output priority -300; policy accept;' \
  'type filter hook forward priority -300; policy accept;' \
  'iifname "lo" accept' \
  'oifname "lo" accept' \
  'counter drop'; do
  grep -qF "$fragment" "$work/applied.nft" || { echo "missing firewall rule: $fragment" >&2; exit 1; }
done

[[ $(run status) == on ]] || { echo "status did not report on" >&2; exit 1; }

: >"$work/log"
run off
[[ ! -e $work/state/enabled ]] || { echo "off did not clear armed state" >&2; exit 1; }
[[ ! -e $work/table ]] || { echo "off did not remove firewall" >&2; exit 1; }
grep -qxF 'nmcli networking on' "$work/log"
grep -qxF 'systemctl disable --quiet ryoku-network-kill-guard.service ryoku-network-kill-disconnect.service' "$work/log"
[[ $(line_of 'nft delete table inet ryoku_kill_switch') -lt $(line_of 'nmcli networking on') ]] || {
  echo "NetworkManager was enabled before firewall was removed" >&2; exit 1;
}
[[ $(run status) == off ]] || { echo "status did not report off" >&2; exit 1; }

: >"$work/log"
run on
: >"$work/log"
run boot
[[ -e $work/table ]] || { echo "boot did not restore firewall" >&2; exit 1; }
grep -qxF 'flush table inet ryoku_kill_switch' "$work/applied.nft" || {
  echo "boot did not atomically refresh an existing firewall table" >&2; exit 1;
}
! grep -qF 'nmcli networking' "$work/log" || { echo "boot must not bring networking up or down" >&2; exit 1; }

: >"$work/log"

if run arbitrary >/dev/null 2>&1; then
  echo "helper accepted an invalid action" >&2
  exit 1
fi
[[ ! -s $work/log ]] || { echo "invalid action ran a privileged command" >&2; exit 1; }

echo "network kill helper checks passed"
