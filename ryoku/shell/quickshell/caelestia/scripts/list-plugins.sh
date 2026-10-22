#!/bin/bash
# Outputs JSON array: [{"id":"..","path":"..","source":"bundled"|"user"}, ...]
BUNDLED="${1:-${XDG_CONFIG_HOME:-$HOME/.config}/quickshell/caelestia/modules/plugins}"
USER="${2:-${XDG_CONFIG_HOME:-$HOME/.config}/caelestia/plugins}"

echo "["
first=1
for source_type in bundled user; do
    [ "$source_type" = "bundled" ] && dir="$BUNDLED" || dir="$USER"
    [ -d "$dir" ] || continue
    for p in "$dir"/*/; do
        [ -f "${p}metadata.json" ] || continue
        name=$(basename "$p")
        path="${p%/}"
        [ $first -eq 1 ] && first=0 || printf ","
        printf '\n  {"id":"%s","path":"%s","source":"%s"}' "$name" "$path" "$source_type"
    done
done
echo ""
echo "]"
