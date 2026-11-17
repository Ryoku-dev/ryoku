#!/usr/bin/env bash
# discover.sh with plugins from stores the user added: each copy has to match a
# receipt from its own store, and a copy that fails that never hides another
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export HOME=$tmp XDG_CONFIG_HOME=$tmp/config XDG_DATA_HOME=$tmp/data XDG_STATE_HOME=$tmp/state
unset RYOSTORE_PLUGINS_DIR RYOKU_PLUGINS_DIR
state=$XDG_STATE_HOME/ryoku/store
digest=$(printf 'a%.0s' {1..64})

fail() { printf 'discover.test: %s\n' "$*" >&2; exit 1; }

# install <store or ""> <id> [receipt source] [receipt destination]
install() {
	local src=$1 id=$2 dir receipt
	local rsrc=${3-$src}
	if [ -z "$src" ]; then
		dir=$XDG_DATA_HOME/ryoku/plugins/$id
		receipt=$state/plugins/$id.json
	else
		dir=$XDG_DATA_HOME/ryoku/sources/$src/plugins/$id
		receipt=$state/sources/$src/plugins/$id.json
	fi
	local dest=${4-${dir#"$XDG_DATA_HOME"/}}
	mkdir -p "$dir" "$(dirname "$receipt")" "$state/plugin-views/$id/$digest"
	printf '{"id":"%s","version":"1.0.0"}\n' "$id" >"$dir/manifest.json"
	cp "$dir/manifest.json" "$state/plugin-views/$id/$digest/manifest.json"
	printf '{"source":"%s","category":"plugins","id":"%s","version":"1.0.0","destination":"%s","files":[]}\n' \
		"$rsrc" "$id" "$dest" >"$receipt"
}

mkdir -p "$XDG_CONFIG_HOME/ryoku" "$state"
install "" official
install acme community
install acme forged "" # a user store receipt claiming to be official
install acme squatter acme ryoku/plugins/squatter
mkdir -p "$XDG_DATA_HOME/ryoku/plugins/shared" # a stale official folder with no receipt
printf '{"id":"shared","version":"1.0.0"}\n' >"$XDG_DATA_HOME/ryoku/plugins/shared/manifest.json"
install beta shared

rows=""
for id in official community forged squatter shared; do
	rows+="${rows:+,}{\"id\":\"$id\",\"version\":\"1.0.0\",\"view\":\"plugin-views/$id/$digest\"}"
done
printf '[%s]\n' "$rows" >"$state/plugins.json"
printf '{"official":{"enabled":true},"community":{"enabled":true},"forged":{"enabled":true},"squatter":{"enabled":true},"shared":{"enabled":true}}\n' \
	>"$XDG_CONFIG_HOME/ryoku/plugins.json"

got=$("$here/discover.sh" | jq -r '[.[].id] | sort | join(" ")')
[ "$got" = "community official shared" ] || fail "enabled plugins are '$got', want 'community official shared'"

dir=$("$here/discover.sh" | jq -r '.[] | select(.id == "shared") | .dir')
[ "$dir" = "$state/plugin-views/shared/$digest" ] || fail "shared runs from '$dir'"

printf '{"official":{"enabled":true}}\n' >"$XDG_CONFIG_HOME/ryoku/plugins.json"
got=$("$here/discover.sh" | jq -r '[.[].id] | join(" ")')
[ "$got" = "official" ] || fail "only official is enabled, got '$got'"

got=$("$here/discover.sh" --all | jq -r '[.[].id] | sort | join(" ")')
[ "$got" = "community official shared" ] || fail "--all lists '$got'"

echo "discover.test: ok"
