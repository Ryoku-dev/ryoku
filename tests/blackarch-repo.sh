#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
export RYOKU_BLACKARCH_LIB=1
# shellcheck source-path=SCRIPTDIR
# shellcheck source=../system/extras/ryoku-pkg-blackarch
source "$root/system/extras/ryoku-pkg-blackarch"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fail() {
  echo "blackarch-repo: $*" >&2
  exit 1
}

assert_file() {
  local got=$1 want=$2 label=$3
  if ! cmp -s "$got" "$want"; then
    diff -u "$want" "$got" >&2 || true
    fail "$label"
  fi
}

conf="$work/pacman.conf"
cat >"$conf" <<'EOF'
[options]
Architecture = auto
SigLevel = Required DatabaseOptional
CacheDir = /var/cache/pacman/pkg/
DBPath = /var/lib/pacman/
HoldPkg = pacman glibc

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist
EOF

blackarch_append_server "$conf" 'https://mirror.example/blackarch'
cat >"$work/append.want" <<'EOF'
[options]
Architecture = auto
SigLevel = Required DatabaseOptional
CacheDir = /var/cache/pacman/pkg/
DBPath = /var/lib/pacman/
HoldPkg = pacman glibc

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist

# added by ryoku-pkg-blackarch
[blackarch]
Server = https://mirror.example/blackarch/$repo/os/$arch
EOF
assert_file "$conf" "$work/append.want" 'managed stanza was not appended last'

cp "$conf" "$work/once"
blackarch_append_server "$conf" 'https://other.example/blackarch'
assert_file "$conf" "$work/once" 'enable edit was not idempotent'
for ending in content blank-line; do
  cat >"$work/roundtrip.original" <<'EOF'
[options]
Architecture = auto

[testing]
Server = https://repo.example/testing/$arch
EOF
  if [[ $ending == blank-line ]]; then
    printf '\n' >>"$work/roundtrip.original"
  fi
  cp "$work/roundtrip.original" "$work/roundtrip.conf"
  blackarch_append_server "$work/roundtrip.conf" 'https://mirror.example/blackarch'
  blackarch_remove_owned "$work/roundtrip.conf"
  assert_file "$work/roundtrip.conf" "$work/roundtrip.original" \
    "enable and disable changed a conf ending with $ending"
done

blackarch_switch_to_include "$conf"
cat >"$work/include.want" <<'EOF'
[options]
Architecture = auto
SigLevel = Required DatabaseOptional
CacheDir = /var/cache/pacman/pkg/
DBPath = /var/lib/pacman/
HoldPkg = pacman glibc

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist

# added by ryoku-pkg-blackarch
[blackarch]
Include = /etc/pacman.d/blackarch-mirrorlist
EOF
assert_file "$conf" "$work/include.want" 'managed stanza did not switch to the packaged mirror list'

blackarch_write_sync_config "$conf" "$work/sync.conf"
cat >"$work/sync.want" <<'EOF'
[options]
Architecture = auto
SigLevel = Required DatabaseOptional
CacheDir = /var/cache/pacman/pkg/
DBPath = /var/lib/pacman/

[blackarch]
Include = /etc/pacman.d/blackarch-mirrorlist
EOF
assert_file "$work/sync.conf" "$work/sync.want" 'sync config included unrelated repositories or dropped required options'

cat >"$conf" <<'EOF'
[options]
Architecture = auto

[blackarch]
Server = https://user.example/blackarch/$repo/os/$arch

# added by ryoku-pkg-blackarch
[blackarch]
Include = /etc/pacman.d/blackarch-mirrorlist
EOF
blackarch_remove_owned "$conf"
cat >"$work/remove.want" <<'EOF'
[options]
Architecture = auto

[blackarch]
Server = https://user.example/blackarch/$repo/os/$arch
EOF
assert_file "$conf" "$work/remove.want" 'disable edit changed a user-written BlackArch stanza'

cp "$conf" "$work/user.once"
blackarch_append_server "$conf" 'https://mirror.example/blackarch'
assert_file "$conf" "$work/user.once" 'enable edit changed an existing user-written stanza'

mkdir "$work/bin"
cat >"$work/bin/ryoku-host" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == pkgmgr ]] || exit 2
echo pacman
EOF
cat >"$work/bin/pacman" <<'EOF'
#!/usr/bin/env bash
[[ ${1:-} == -Qq && ${2:-} == blackarch-keyring ]]
EOF
chmod +x "$work/bin/ryoku-host" "$work/bin/pacman"
status=$(
  unset RYOKU_BLACKARCH_LIB
  PATH="$work/bin:$PATH" RYOKU_PACMAN_CONF="$conf" \
    "$root/system/extras/ryoku-pkg-blackarch" status --json
)
[[ $status == '{"enabled":true,"ours":false,"keyring":true}' ]] ||
  fail "status ignored RYOKU_PACMAN_CONF: $status"

echo 'blackarch-repo: ok'
