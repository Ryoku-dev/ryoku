#!/usr/bin/env bash
# Keeps better-blur-dx's window class list in sync with the shell.
#
# better-blur-dx only blurs windows matching (or, in exclusive mode, not
# matching) its class list, and Quickshell's own surfaces have to be on that
# list for the shell's blur to work while the effect is enabled. Depending
# on how the effect was configured, that means adding or removing the
# quickshell class, then reconfiguring KWin so the change lands live.
#
# Prints BBDX_ENABLED when the effect is enabled, so a caller that only
# wants the effect's state can read it from stdout without repeating the
# kreadconfig6 query.
#
# Both shell.qml (startup guard) and the Nexus appearance page (after the
# user toggles the shell's own blur) run this.

IS_ENABLED=$(kreadconfig6 --file kwinrc --group Plugins --key better_blur_dxEnabled)
if [ "$IS_ENABLED" = "true" ]; then
    BLUR_MATCHING=$(kreadconfig6 --file kwinrc --group Effect-better-blur-dx --key BlurMatching)
    BLUR_NON_MATCHING=$(kreadconfig6 --file kwinrc --group Effect-better-blur-dx --key BlurNonMatching)
    WINDOW_CLASSES=$(kreadconfig6 --file kwinrc --group Effect-better-blur-dx --key WindowClasses)

    if [ -z "$BLUR_MATCHING" ]; then BLUR_MATCHING="true"; fi
    if [ -z "$BLUR_NON_MATCHING" ]; then BLUR_NON_MATCHING="false"; fi

    MODIFIED=false

    if [ "$BLUR_MATCHING" = "true" ] && [ "$BLUR_NON_MATCHING" = "false" ]; then
        if echo "$WINDOW_CLASSES" | grep -q '\bquickshell\b'; then
            # Remove quickshell without destroying the rest of the line if comma-separated
            NEW_CLASSES=$(echo "$WINDOW_CLASSES" | sed -E 's/\bquickshell\b//g' | sed 's/,,/,/g' | sed 's/^,//' | sed 's/,$//')
            kwriteconfig6 --file kwinrc --group Effect-better-blur-dx --key WindowClasses "$NEW_CLASSES"
            MODIFIED=true
        fi
    elif [ "$BLUR_MATCHING" = "false" ] && [ "$BLUR_NON_MATCHING" = "true" ]; then
        if ! echo "$WINDOW_CLASSES" | grep -q '\bquickshell\b'; then
            if [ -z "$WINDOW_CLASSES" ]; then
                NEW_CLASSES="quickshell"
            elif echo "$WINDOW_CLASSES" | grep -q ','; then
                NEW_CLASSES="$WINDOW_CLASSES,quickshell"
            else
                NEW_CLASSES="$WINDOW_CLASSES"$'\n'"quickshell"
            fi
            kwriteconfig6 --file kwinrc --group Effect-better-blur-dx --key WindowClasses "$NEW_CLASSES"
            MODIFIED=true
        fi
    fi

    if [ "$MODIFIED" = "true" ]; then
        qdbus6 org.kde.KWin /KWin reconfigure 2>/dev/null || true
        qdbus6 org.kde.KWin /Effects reconfigureEffect better_blur_dx 2>/dev/null || true
    fi

    echo "BBDX_ENABLED"
fi
