#!/usr/bin/env bash

# Python's path contract for its shell scripts: the vendored payload sits at
# the barstyle's module root, state lives under the ryoku namespaces, and the
# settings file is Ryoku's own shell.json (read here, written only by the
# ryoku-shell daemon).

if [ -z "$RYOKU_PYTHON_DIR" ]; then
    SCRIPT_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
    PY_DIR="$(dirname "$SCRIPT_DIR")"
    export RYOKU_PYTHON_DIR="$PY_DIR"
fi

export SERPANTINUM_DIR="$RYOKU_PYTHON_DIR"
export QS_DIR="$RYOKU_PYTHON_DIR"
export QS_PAYLOAD="$RYOKU_PYTHON_DIR"
export MAIN_QML="$RYOKU_PYTHON_DIR/Scene.qml"
export SHELL_INSTANCE="shell"

export QS_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/ryoku/python"
export QS_STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/ryoku/python"
export QS_RUN_DIR="${XDG_RUNTIME_DIR:-/tmp}/ryoku-python"
export QS_LOG_DIR="$QS_RUN_DIR/logs"
export QS_SETTINGS="${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/shell.json"

[[ -d "$QS_LOG_DIR" && -d "$QS_CACHE_DIR" && -d "$QS_STATE_DIR" ]] || mkdir -p "$QS_CACHE_DIR" "$QS_STATE_DIR" "$QS_RUN_DIR" "$QS_LOG_DIR"

qs_ensure_cache() {
    local WIDGET_NAME="$1"

    local WIDGET_UPPER="${WIDGET_NAME^^}"

    local WIDGET_CACHE="$QS_CACHE_DIR/$WIDGET_NAME"
    local WIDGET_STATE="$QS_STATE_DIR/$WIDGET_NAME"
    local WIDGET_RUN="$QS_RUN_DIR/$WIDGET_NAME"

    [[ -d "$WIDGET_RUN" && -d "$WIDGET_STATE" && -d "$WIDGET_CACHE" ]] || mkdir -p "$WIDGET_CACHE" "$WIDGET_STATE" "$WIDGET_RUN"

    export "QS_CACHE_${WIDGET_UPPER}=$WIDGET_CACHE"
    export "QS_STATE_${WIDGET_UPPER}=$WIDGET_STATE"
    export "QS_RUN_${WIDGET_UPPER}=$WIDGET_RUN"
}

if [ -d "$QS_DIR" ]; then
    for dir in "$QS_DIR"/*/; do
        [ -d "$dir" ] || continue

        dir_trimmed="${dir%/}"
        WIDGET_NAME="${dir_trimmed##*/}"

        qs_ensure_cache "$WIDGET_NAME"
    done
fi

qs_ensure_cache "focustime"

if [ -z "${SERPANTINUM_INSTALLER:-}" ] && [ ! -f "$QS_STATE_DIR/version" ]; then
    ver=""
    if [ -f "$SERPANTINUM_DIR/version.txt" ]; then
        ver="$(cat "$SERPANTINUM_DIR/version.txt" 2>/dev/null | xargs)"
    elif [ -f "$(dirname "$SERPANTINUM_DIR")/version.txt" ]; then
        ver="$(cat "$(dirname "$SERPANTINUM_DIR")/version.txt" 2>/dev/null | xargs)"
    fi
    [ -z "$ver" ] && ver="${SERPANTINUM_VERSION:-2.0.0}"
    echo "SERPANTINUM_VERSION=\"$ver\"" > "$QS_STATE_DIR/version"
fi

[ -f "$QS_STATE_DIR/quickactions/palettes.json" ] || echo "[]" > "$QS_STATE_DIR/quickactions/palettes.json"
