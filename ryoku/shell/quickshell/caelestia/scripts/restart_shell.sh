#!/bin/bash

# Restart the shell through the systemd user unit the install wrote. Restarting
# that unit keeps the restart environment identical to login startup, which is the
# whole point of going through it.
#
# One unit name for both install kinds: the package ships
# /usr/lib/systemd/user/caelestia-shell.service, and 10-autostart.sh writes the
# same unit for a checkout with the checkout's paths in it. It replaced the
# desktop entry that KDE's xdg-autostart generator turned into
# app-caelestiashell@autostart.service, so one mechanism starts the shell instead
# of two racing each other.
#
# Do not replace this with the CLI's `caelestia shell -d`: that daemonises,
# which points the shell's stdio at /dev/null, and every application launched
# from the shell then inherits a stdout that goes nowhere. Vesktop deadlocks
# when a call starts in exactly that state (issue #402, reproducible with
# `vesktop >/dev/null 2>&1`).
#
# Callers: the shell's own restart actions (PluginsPage, AppearancePage,
# Toggles) and update.sh.

if command -v systemctl >/dev/null 2>&1; then
    exec systemctl --user restart caelestia-shell.service
fi

exit 1

