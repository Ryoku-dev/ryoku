#!/usr/bin/env bash
# Hermetic checks for ryoku-dns (system/hardware/network/ryoku-dns), the system
# DNS switch behind the bar's DNS picker. The NetworkManager and
# systemd-resolved drop-ins are temp files, and ryoku-host, nmcli and
# resolvectl are stubs that log their calls, so nothing on the host changes.
set -euo pipefail
ROOT=${RYOKU_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
helper="$ROOT/system/hardware/network/ryoku-dns"
[[ -x $helper ]] || { echo "missing executable helper: $helper" >&2; exit 1; }

fail() { echo "ryoku-dns: $*" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
cat >"$work/bin/ryoku-host" <<'EOF'
#!/usr/bin/env bash
printf 'ryoku-host %s\n' "$*" >>"$RYOKU_TEST_LOG"
if [[ $* == "svc --system is-active systemd-resolved" ]]; then
  [[ $RYOKU_TEST_RESOLVED == 1 ]]
elif [[ $* == "svc --system is-active NetworkManager" ]]; then
  [[ ${RYOKU_TEST_NM:-1} == 1 ]]
fi
EOF
cat >"$work/bin/nmcli" <<'EOF'
#!/usr/bin/env bash
printf 'nmcli %s\n' "$*" >>"$RYOKU_TEST_LOG"
if [[ $1 == -g ]]; then
  printf 'wlan0:connected\nenp0s13f0u4:connected\ntailscale0:unmanaged\nlo:unmanaged\n'
fi
EOF
cat >"$work/bin/resolvectl" <<'EOF'
#!/usr/bin/env bash
printf 'resolvectl %s\n' "$*" >>"$RYOKU_TEST_LOG"
if [[ $1 == dns ]]; then
  printf 'Global:\nLink 2 (wlan0): 192.168.1.1\nLink 4 (enp0s13f0u4): 192.168.1.1\nLink 7 (tailscale0): 100.100.100.100\n'
fi
EOF
chmod +x "$work/bin/ryoku-host" "$work/bin/nmcli" "$work/bin/resolvectl"

mkdir -p "$work/resolve"
: >"$work/resolve/stub-resolv.conf"
ln -s "$work/resolve/stub-resolv.conf" "$work/resolv.conf"
nm="$work/nm/20-ryoku-dns.conf"
res="$work/resolved/20-ryoku-dns.conf"
run() {
  local resolved=$1
  shift
  : >"$work/log"
  PATH="$work/bin:$PATH" \
  RYOKU_DNS_TEST=1 \
  RYOKU_DNS_NM_CONF="$nm" \
  RYOKU_DNS_RESOLVED_CONF="$res" \
  RYOKU_DNS_HOST="$work/bin/ryoku-host" \
  RYOKU_DNS_RESOLV_CONF="$work/resolv.conf" \
  RYOKU_DNS_RESOLVE_DIR="$work/resolve" \
  RYOKU_TEST_RESOLVED="$resolved" \
  RYOKU_TEST_LOG="$work/log" \
    "$helper" "$@"
}
logged() { grep -qxF -- "$1" "$work/log"; }
# order LINE-A LINE-B: A was logged before B
order() {
  local a b
  a=$(grep -nxF -- "$1" "$work/log" | head -1 | cut -d: -f1)
  b=$(grep -nxF -- "$2" "$work/log" | tail -1 | cut -d: -f1)
  [[ -n $a && -n $b ]] && (( a < b ))
}

# systemd-resolved: NetworkManager stops pushing connection DNS and the servers
# become resolved's global DNS, still recorded as [global-dns] for the daemon.
run 1 cloudflare
grep -qx 'dns=none' "$nm" || fail "NetworkManager still pushes connection DNS under resolved"
grep -qx 'servers=1.1.1.1,1.0.0.1,2606:4700:4700::1111,2606:4700:4700::1001' "$nm" ||
  fail "[global-dns] does not record the Cloudflare servers"
grep -qx 'DNS=1.1.1.1 1.0.0.1 2606:4700:4700::1111 2606:4700:4700::1001' "$res" ||
  fail "resolved does not get the Cloudflare servers"
grep -qx 'Domains=~.' "$res" || fail "resolved does not route every domain to the global servers"
if ! logged "resolvectl revert -- wlan0" || ! logged "resolvectl revert -- enp0s13f0u4"; then
  fail "the DNS NetworkManager pushed is left on the links"
fi
! logged "resolvectl revert -- tailscale0" || fail "a link NetworkManager does not manage was reverted"
order "nmcli general reload dns-full" "resolvectl revert -- wlan0" ||
  fail "links reverted before NetworkManager stopped pushing DNS"
order "resolvectl revert -- enp0s13f0u4" "ryoku-host svc --system reload systemd-resolved" ||
  fail "resolved reloaded before the links were reverted"

# back to DHCP: both drop-ins go, resolved drops its globals before
# NetworkManager pushes connection DNS again.
run 1 dhcp
[[ ! -e $nm && ! -e $res ]] || fail "dhcp left a drop-in behind"
order "ryoku-host svc --system reload systemd-resolved" "nmcli general reload dns-full" ||
  fail "resolved kept its globals while NetworkManager pushed connection DNS"
! grep -q '^resolvectl revert' "$work/log" || fail "dhcp reverted links NetworkManager is about to fill"

# without resolved: only NetworkManager [global-dns], as before.
run 0 google
grep -qx 'servers=8.8.8.8,8.8.4.4,2001:4860:4860::8888,2001:4860:4860::8844' "$nm" ||
  fail "[global-dns] does not record the Google servers"
! grep -q 'dns=none' "$nm" || fail "dns=none written without resolved"
[[ ! -e $res ]] || fail "a resolved drop-in written without resolved"
! grep -q '^resolvectl' "$work/log" || fail "resolvectl called without resolved"
run 0 dhcp

# resolved running but NetworkManager not feeding it (resolv.conf is not
# resolved's): changing DNS there would leave resolv.conf stale, so only
# [global-dns] is written.
rm "$work/resolv.conf"
: >"$work/resolv.conf"
run 1 cloudflare
! grep -q 'dns=none' "$nm" || fail "dns=none written though NetworkManager does not use resolved"
[[ ! -e $res ]] || fail "a resolved drop-in written though NetworkManager does not use resolved"
! grep -q '^resolvectl' "$work/log" || fail "resolvectl called though NetworkManager does not use resolved"
run 1 dhcp
rm "$work/resolv.conf"
ln -s "$work/resolve/stub-resolv.conf" "$work/resolv.conf"

# no NetworkManager (networkd + resolved): nothing to take over from, so no
# resolved drop-in either.
RYOKU_TEST_NM=0 run 1 google
[[ ! -e $res ]] || fail "a resolved drop-in written without NetworkManager"
! grep -q '^resolvectl' "$work/log" || fail "resolvectl called without NetworkManager"
RYOKU_TEST_NM=0 run 1 dhcp

# custom servers, and a bad one refused before anything changes.
run 1 custom 9.9.9.9 149.112.112.112
grep -qx 'DNS=9.9.9.9 149.112.112.112' "$res" || fail "custom servers not given to resolved"
cp "$res" "$work/before"
if run 1 custom 9.9.9.9 300.1.1.1 2>/dev/null; then
  fail "an invalid server was accepted"
fi
if run 1 custom 08.1.1.1 2>/dev/null; then
  fail "an octal-looking server was accepted"
fi
if run 1 custom 'x;y' 2>/dev/null; then
  fail "a server carrying shell syntax was accepted"
fi
cmp -s "$res" "$work/before" || fail "a refused change rewrote the drop-in"
run 1 dhcp

echo "ryoku-dns: ok"
