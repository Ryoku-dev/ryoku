#!/usr/bin/env bash
# Migration 023: Switch cliphist watcher to type-specific watchers
#
# The old `wl-paste --watch cliphist store` stores every MIME type the
# clipboard offers. Browsers offer both text/html and text/plain, so
# cliphist gets duplicate entries (one with HTML tags, one clean).
#
# Replace with two type-specific watchers:
#   wl-paste --type text --watch cliphist store
#   wl-paste --type image --watch cliphist store

MIGRATION_ID="023-cliphist-typed-watchers"
MIGRATION_TITLE="Switch cliphist to type-specific watchers (fix duplicate entries)"
MIGRATION_DESCRIPTION="Replaces the single untyped 'wl-paste --watch cliphist store' with separate text and image watchers to prevent duplicate clipboard entries from browsers."
MIGRATION_TARGET_FILE="~/.config/niri/config.d/50-startup.kdl"
MIGRATION_REQUIRED=true

_cliphist_startup_file="${HOME}/.config/niri/config.d/50-startup.kdl"

migration_check() {
    # Not needed if startup file doesn't exist
    [[ -f "$_cliphist_startup_file" ]] || return 1

    # Already migrated — has typed watcher
    if grep -q 'wl-paste --type text --watch cliphist store' "$_cliphist_startup_file" 2>/dev/null; then
        return 1
    fi

    # Needs migration if untyped watcher is present
    if grep -q 'wl-paste --watch cliphist store' "$_cliphist_startup_file" 2>/dev/null; then
        return 0
    fi

    # No watcher at all — nothing to migrate
    return 1
}

migration_preview() {
    echo -e "${STY_RED}- wl-paste --watch cliphist store${STY_RST}"
    echo -e "${STY_GREEN}+ wl-paste --type text --watch cliphist store${STY_RST}"
    echo -e "${STY_GREEN}+ wl-paste --type image --watch cliphist store${STY_RST}"
    echo ""
    echo "Prevents duplicate clipboard entries when copying from browsers."
}

migration_apply() {
    [[ -f "$_cliphist_startup_file" ]] || return 1

    # Replace untyped watcher with text-only watcher
    sed -i \
        's|wl-paste --watch cliphist store|wl-paste --type text --watch cliphist store|' \
        "$_cliphist_startup_file"

    # Add the image watcher right after the text watcher line
    sed -i \
        '/wl-paste --type text --watch cliphist store/a spawn-at-startup "bash" "-c" "wl-paste --type image --watch cliphist store \&"' \
        "$_cliphist_startup_file"

    # Verify the migration applied
    grep -q 'wl-paste --type text --watch cliphist store' "$_cliphist_startup_file"
}
