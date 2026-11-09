#!/usr/bin/env bash
# Runs fixed palette cases through the Go, web and QML resolvers and fails when
# any stack paints a role differently. Run it after touching tokens.json's palette
# groups, palette/*.go, web/palette.js or Lithic.qml.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
library=$(dirname "$here")
js=${LITHIC_JS:-node}
qml=${LITHIC_QML:-qml6}
edition=$(sed -n 's/^ID=//p' /etc/os-release 2>/dev/null | tr -d '"')
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

(cd "$library/go" && go build -o "$work/lithic-roles" ./cmd/lithic-roles)

failed=0

# run <case> <mode> <web input file, or - when the case is a loader rule a browser never sees>
run() {
	local name=$1 mode=$2 web=$3 dir=$work/$1
	export XDG_CONFIG_HOME=$dir/config XDG_CACHE_HOME=$dir/cache
	"$work/lithic-roles" --mode "$mode" >"$dir/go-$mode.json"
	QT_FORCE_STDERR_LOGGING=1 QT_QPA_PLATFORM=offscreen QML_XHR_ALLOW_FILE_READ=1 LITHIC_MODE=$mode \
		timeout 30 "$qml" -I "$library/qml" "$here/Roles.qml" 2>&1 |
		sed -n 's/.*ROLES //p' >"$dir/qml-$mode.json"
	local sets=("go=$dir/go-$mode.json" "qml=$dir/qml-$mode.json")
	if [[ $web != - ]]; then
		"$js" "$here/roles.mjs" "$web" "$mode" >"$dir/web-$mode.json"
		sets+=("web=$dir/web-$mode.json")
	fi
	"$js" "$here/compare.mjs" "$name $mode" "${sets[@]}" || failed=1
}

# fresh <case>: an empty config and cache tree for one case
fresh() { mkdir -p "$work/$1/config/ryoku" "$work/$1/cache/ryoku"; }

for name in wallpaper-dark wallpaper-light; do
	fresh "$name"
	cp "$here/testdata/$name.json" "$work/$name/cache/ryoku/colors.json"
	for mode in follow dark light; do run "$name" "$mode" "$here/testdata/$name.json"; done
done

fresh scheme
printf '{"themePalette": %s}\n' "$(cat "$here/testdata/scheme.json")" >"$work/scheme/config/ryoku/shell.json"
for mode in follow dark light; do run scheme "$mode" "$work/scheme/config/ryoku/shell.json"; done

fresh signature
printf '{"roles": {}, "source": "signature", "edition": "%s"}\n' "$edition" >"$work/signature/web.json"
for mode in follow dark light; do run signature "$mode" "$work/signature/web.json"; done

fresh unfollowed
printf '{"followWallpaper": false}\n' >"$work/unfollowed/config/ryoku/theme.json"
cp "$here/testdata/wallpaper-dark.json" "$work/unfollowed/cache/ryoku/colors.json"
for mode in follow dark light; do run unfollowed "$mode" -; done

exit "$failed"
