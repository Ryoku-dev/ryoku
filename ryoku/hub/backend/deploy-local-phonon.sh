#!/bin/sh
set -eu
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
target="$HOME/.local/libexec/ryoku-hub-phonon"
launcher="$HOME/.local/bin/ryoku-hub"
if [ -e "$launcher" ] && ! cmp -s "$launcher" "$here/local-phonon-hub"; then
    printf '%s\n' "Refusing to replace an unrelated local ryoku-hub launcher: $launcher" >&2
    exit 1
fi
mkdir -p "$HOME/.local/libexec" "$HOME/.local/bin"
stage=$(mktemp -d "$HOME/.local/libexec/.phonon-build-XXXXXX")
trap 'rmdir "$stage" 2>/dev/null || true' EXIT
(cd "$here" && go build -o "$stage/ryoku-hub-phonon" .)
chmod 755 "$stage/ryoku-hub-phonon"
mv "$stage/ryoku-hub-phonon" "$target"
install -m755 "$here/local-phonon-hub" "$launcher"
