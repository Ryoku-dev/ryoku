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

# run <case> <mode> <web input file, or - when the case is a loader rule a browser never sees> [contrast] [tint] [design]
run() {
	local name=$1 mode=$2 web=$3 contrast=${4:-standard} tint=${5:-off} design=${6:-lithic} dir=$work/$1
	local look=$mode-$contrast-$tint label="$1 $2 ${4:-standard} ${5:-off}"
	if [[ $design != lithic ]]; then
		look+=-$design
		label+=" $design"
	fi
	export XDG_CONFIG_HOME=$dir/config XDG_CACHE_HOME=$dir/cache
	"$work/lithic-roles" --mode "$mode" --contrast "$contrast" --tint "$tint" --design "$design" >"$dir/go-$look.json"
	QT_FORCE_STDERR_LOGGING=1 QT_QPA_PLATFORM=offscreen QML_XHR_ALLOW_FILE_READ=1 LITHIC_MODE=$mode \
		LITHIC_CONTRAST=$contrast LITHIC_TINT=$tint LITHIC_DESIGN=$design LITHIC_PALETTE_JSON='' \
		timeout 30 "$qml" -I "$library/qml" "$here/Roles.qml" 2>&1 |
		sed -n 's/.*ROLES //p' >"$dir/qml-$look.json"
	local sets=("go=$dir/go-$look.json" "qml=$dir/qml-$look.json")
	if [[ $web != - ]]; then
		"$js" "$here/roles.mjs" "$web" "$mode" "$contrast" "$tint" >"$dir/web-$look.json"
		sets+=("web=$dir/web-$look.json")
	fi
	"$js" "$here/compare.mjs" "$label" "${sets[@]}" || failed=1
}

# fresh <case>: an empty config and cache tree for one case
fresh() { mkdir -p "$work/$1/config/ryoku" "$work/$1/cache/ryoku"; }

# carried <case> <roles file> <source>: the palette a browser receives, with this host's edition
carried() {
	printf '{"roles": %s, "source": "%s", "edition": "%s"}\n' "$(cat "$2")" "$3" "$edition" >"$work/$1/web.json"
	echo "$work/$1/web.json"
}

# stock <case> <mode> [design]: Go paints exactly the signature case's roles for this mode
stock() {
	local look=$2-standard-off${3:+-$3}
	"$js" "$here/compare.mjs" "$1 $2${3:+ $3} is the signature" "signature=$work/signature/go-$2-standard-off.json" \
		"$1=$work/$1/go-$look.json" || failed=1
}

fresh signature
printf '{"roles": {}, "source": "signature", "edition": "%s"}\n' "$edition" >"$work/signature/web.json"
for mode in dark light follow; do run signature "$mode" "$work/signature/web.json"; done
for mode in light follow; do run signature "$mode" "$work/signature/web.json" high; done

for name in wallpaper-dark wallpaper-light; do
	fresh "$name"
	cp "$here/testdata/$name.json" "$work/$name/cache/ryoku/colors.json"
	web=$(carried "$name" "$here/testdata/$name.json" wallpaper)
	for mode in dark light; do run "$name" "$mode" "$web" && stock "$name" "$mode"; done
	run "$name" follow "$here/testdata/$name.json"
done
run wallpaper-dark follow "$here/testdata/wallpaper-dark.json" high medium
run wallpaper-light follow "$here/testdata/wallpaper-light.json" high low

fresh scheme
printf '{"themePalette": %s}\n' "$(cat "$here/testdata/scheme.json")" >"$work/scheme/config/ryoku/shell.json"
web=$(carried scheme "$here/testdata/scheme.json" scheme)
for mode in dark light; do run scheme "$mode" "$web" && stock scheme "$mode"; done
run scheme follow "$work/scheme/config/ryoku/shell.json"
run scheme follow "$work/scheme/config/ryoku/shell.json" standard low

fresh unfollowed
printf '{"followWallpaper": false}\n' >"$work/unfollowed/config/ryoku/theme.json"
cp "$here/testdata/wallpaper-dark.json" "$work/unfollowed/cache/ryoku/colors.json"
web=$(carried unfollowed "$here/testdata/wallpaper-dark.json" wallpaper)
for mode in dark light; do run unfollowed "$mode" "$web" && stock unfollowed "$mode"; done
run unfollowed follow - && stock unfollowed follow
run unfollowed dark "$web" standard low

# The round designs on the desktop's colours wear its Material roles as they are
# (Lithic.qml _materialFrom); on Ryoku's colours they paint the signature like Lithic.
# The web resolver has no designs, so these compare Go with QML.
for design in material adwaita fluent; do
	for mode in dark light; do run wallpaper-dark "$mode" - standard off "$design" && stock wallpaper-dark "$mode" "$design"; done
	run wallpaper-dark follow - standard off "$design"
	run wallpaper-light follow - standard off "$design"
	run scheme follow - standard off "$design"
	run unfollowed follow - standard off "$design" && stock unfollowed follow "$design"
done
# contrast and tint reach only scrim and the minerals there
run wallpaper-dark follow - high medium material
run wallpaper-light follow - high low adwaita
# a palette missing roles keeps the toned ladder for them, the container among them
fresh material-partial
cp "$here/testdata/material-partial.json" "$work/material-partial/cache/ryoku/colors.json"
for design in lithic material fluent; do run material-partial follow - standard off "$design"; done

exit "$failed"
