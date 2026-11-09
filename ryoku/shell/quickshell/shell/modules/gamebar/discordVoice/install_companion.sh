#!/usr/bin/env bash
# Build the Ryoku voice companion into Vesktop or Equibop. Both ship Vencord
# without a userplugin loader, so the plugin is compiled into a source build and
# the client's custom-mod setting points at dist. Close the client first.
#
#   install_companion.sh [--client auto|vesktop|equibop]
set -euo pipefail

DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN="$HERE/companion"
PLUGIN_NAME="ryokuDiscordVoice"
INSTALL_ROOT="$DATA_HOME/ryoku/discord-voice"

CLIENT="auto"
if [[ "${1:-}" == "--client" ]]; then
    CLIENT="${2:?--client needs auto, vesktop or equibop}"
fi

say() { printf '  %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

case "$CLIENT" in
    auto|vesktop|equibop) ;;
    *) die "unsupported client: $CLIENT (use auto, vesktop or equibop)" ;;
esac

[[ -d "$PLUGIN" ]] || die "companion source is missing at $PLUGIN"
command -v git >/dev/null || die "git is required"
command -v node >/dev/null || die "node is required"
command -v npm >/dev/null || die "npm is required"

PNPM_PREFIX="$HOME/.local"
export PATH="$PNPM_PREFIX/bin:$PATH"
if ! command -v pnpm >/dev/null; then
    if command -v corepack >/dev/null; then
        say "enabling pnpm through corepack"
        corepack enable pnpm
    else
        say "installing pnpm into the user prefix"
        npm install --global --prefix "$PNPM_PREFIX" pnpm@10
        hash -r
    fi
fi
command -v pnpm >/dev/null || die "pnpm could not be installed"

client_repo() {
    case "$1" in
        vesktop) printf 'https://github.com/Vendicated/Vencord.git' ;;
        equibop) printf 'https://github.com/Equicord/Equicord.git' ;;
    esac
}

client_dir() {
    case "$1" in
        vesktop) printf '%s/Vencord' "$INSTALL_ROOT" ;;
        equibop) printf '%s/Equicord' "$INSTALL_ROOT" ;;
    esac
}

client_state() {
    case "$1" in
        vesktop) printf '%s/vesktop/state.json' "$CONFIG_HOME" ;;
        equibop) printf '%s/equibop/state.json' "$CONFIG_HOME" ;;
    esac
}

client_key() {
    case "$1" in
        vesktop) printf 'vencordDir' ;;
        equibop) printf 'equicordDir' ;;
    esac
}

client_running() {
    case "$1" in
        vesktop) pgrep -x vesktop >/dev/null || pgrep -f 'vesktop --' >/dev/null ;;
        equibop) pgrep -x equibop >/dev/null || pgrep -f 'equibop --' >/dev/null ;;
    esac
}

install_client() {
    local client="$1"
    local repo dir state key

    repo="$(client_repo "$client")"
    dir="$(client_dir "$client")"
    state="$(client_state "$client")"
    key="$(client_key "$client")"

    if client_running "$client"; then
        say "$client is running; close it fully (including the tray), then run this again."
        return 1
    fi

    say "$client mod source: $dir"
    if [[ -d "$dir/.git" ]]; then
        local origin current dirty
        origin="$(git -C "$dir" remote get-url origin 2>/dev/null || true)"
        [[ "$origin" == "$repo" ]] || die "$dir tracks ${origin:-no remote}, not $repo; move it aside and run this again"
        current="$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
        [[ "$current" == "main" ]] || die "$dir is on ${current:-an unknown branch}, not main; check it out and run this again"
        dirty="$(git -C "$dir" status --porcelain --untracked-files=no)"
        [[ -z "$dirty" ]] || die "$dir has local changes; revert them and run this again"
        git -C "$dir" pull --ff-only || die "could not update $dir; check the network and run this again"
    else
        mkdir -p "$(dirname "$dir")"
        git clone --depth 1 "$repo" "$dir"
    fi

    say "copying the $PLUGIN_NAME user plugin"
    mkdir -p "$dir/src/userplugins"
    rm -rf "$dir/src/userplugins/$PLUGIN_NAME"
    cp -r "$PLUGIN" "$dir/src/userplugins/$PLUGIN_NAME"

    say "building $client (this takes a few minutes the first time)"
    (cd "$dir" && pnpm install --frozen-lockfile && pnpm build)
    cp "$dir/package.json" "$dir/dist/package.json"

    if [[ ! -f "$state" ]]; then
        say "no $state found; set the custom mod location to $dir/dist by hand"
        return 0
    fi

    say "pointing $client at the custom build"
    [[ -f "$state.pre-ryoku-voice" ]] || cp "$state" "$state.pre-ryoku-voice"
    python3 - "$state" "$key" "$dir/dist" <<'PY'
import json
import sys

path, key, dist = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "r", encoding="utf-8") as handle:
    state = json.load(handle)
state[key] = dist
with open(path, "w", encoding="utf-8") as handle:
    json.dump(state, handle, indent=4)
    handle.write("\n")
PY

    say "done; start $client and RyokuDiscordVoice is enabled by default"
    say "the Voice card connects on its own once the shell is running"
}

clients=()
if [[ "$CLIENT" == "auto" ]]; then
    if client_running vesktop; then
        clients=("vesktop")
    elif client_running equibop; then
        clients=("equibop")
    elif [[ -d "$CONFIG_HOME/vesktop" ]]; then
        clients=("vesktop")
    elif [[ -d "$CONFIG_HOME/equibop" ]]; then
        clients=("equibop")
    else
        die "no supported client config found; pass --client"
    fi
else
    clients=("$CLIENT")
fi

status=0
for client in "${clients[@]}"; do
    if (set -e; install_client "$client"); then
        :
    else
        status=1
    fi
done
exit "$status"
