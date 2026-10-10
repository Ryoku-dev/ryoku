-- The Hyprland provider's login bootstrap; a second compositor ships its own.
hl.on("hyprland.start", function()
    -- Start the GNOME keyring's secrets + pkcs11 agents before anything that
    -- might ask for a stored secret. Idempotent: if PAM already started it at
    -- login (unlock-on-login mode), this just re-prints its env and exits.
    hl.exec_cmd("gnome-keyring-daemon --start --components=secrets,pkcs11")
    hl.exec_cmd("ryoku-wm-hyprland act cursor.set Bibata-Modern-Ice 24")
    hl.exec_cmd("gsettings set org.gnome.desktop.interface color-scheme prefer-dark")
    -- adw-gtk3-dark, not Adwaita-dark: stock Adwaita GTK3 hardcodes its colours,
    -- so the palette Ryoku generates barely reaches GTK3 apps, while adw-gtk3
    -- derives its rules from the named colours we already emit. The daemon owns
    -- accent-color; do not set it here.
    hl.exec_cmd("gsettings set org.gnome.desktop.interface gtk-theme adw-gtk3-dark")
    -- Folder icons follow the wallpaper accent: ryoku-cmd-folders builds a small
    -- Papirus-Dark overlay under ~/.local/share/icons tinted to the palette and
    -- selects it. Rebuilt on every palette change by the shell's matugen hook.
    hl.exec_cmd("command -v ryoku-cmd-folders >/dev/null 2>&1 && ryoku-cmd-folders")
    -- ONE chained command: exec is fire-and-forget, so as separate lines the
    -- shell start races the env import, and losing means
    -- ConditionEnvironment=WAYLAND_DISPLAY skips the unit (a black desktop).
    -- --all, not a name list: a keybind-launched app is Hyprland's child and
    -- sees every hl.env() name, a systemd or D-Bus launched one sees only what
    -- is pushed here, and a hand-kept list drifts the moment env.lua, the Hub,
    -- or user.lua adds one.
    -- restart, not start: the user manager outlives a session (linger, a
    -- relogin after a compositor crash), and a daemon it still holds from the
    -- previous one answers `start` with "already active" while its surfaces are
    -- bound to the dead compositor, so the login lands on bare Hyprland with no
    -- shell. daemon-reload first, so a freshly materialized unit is visible.
    -- Raise the reload cover before anything else touches the session: the
    -- greeter's unlock animation has just ended, the shell services restart
    -- below, and without the cover the desktop loads grey and flickers while
    -- every QML surface comes up. begin blocks until the cover is painted
    -- (or fails fast headless), then the session chain proceeds behind it.
    -- session-start owns the guarded service restart and does not return until
    -- the shell and lid owners have proved readiness; the fresh shell finishes
    -- the cover once wallpaper and desktop report ready on every screen.
    -- Portals restart last:
    -- they are only PartOf=graphical-session.target and a stale frontend would
    -- otherwise proxy every ScreenCast request to the dead session backend.
    hl.exec_cmd("dbus-update-activation-environment --systemd --all; systemctl --user daemon-reload; ryoku-reload-cover begin boot || true; ryoku-power-cutover session-start-logged && systemctl --user try-restart xdg-desktop-portal.service xdg-desktop-portal-hyprland.service xdg-desktop-portal-gtk.service")
    -- Polkit authentication is answered by the shell's own agent (the island
    -- that matches the rest of the desktop), so the stock Qt agent must not
    -- take the session's single agent slot. Stopping it is idempotent and
    -- harmless when it was never started.
    hl.exec_cmd("systemctl --user stop hyprpolkitagent 2>/dev/null || true")
    hl.exec_cmd("command -v ryoku-monitor >/dev/null 2>&1 && ryoku-monitor autoscale")
    -- Per-device Bluetooth codec memory. WirePlumber restores a device's PROFILE
    -- across reconnects but not its A2DP codec, so a user who picked one has to
    -- pick it again every single time the buds come back. `watch` reconciles on
    -- every card event and is a no-op for anyone who never chose a codec, so it
    -- costs a subscription and nothing else.
    hl.exec_cmd("command -v ryoku-bt-audio >/dev/null 2>&1 && ryoku-bt-audio watch")
    -- Keyring default: no app ever prompts for a keyring password out of the box.
    -- `keyring init` records the mode once and seeds a blank passwordless default
    -- keyring for never-ask (idempotent; a no-op once a mode is chosen). Best
    -- effort: an old ryoku without the subcommand just fails silently here.
    hl.exec_cmd("command -v ryoku >/dev/null 2>&1 && ryoku keyring init")
    hl.exec_cmd("command -v ryoku-gpu >/dev/null 2>&1 && ryoku-gpu persist")
    -- Power knobs the kernel forgets across a reboot: the battery charge ceiling
    -- and the PCIe link policy are plain sysfs values, so a stored choice has to
    -- be pushed back at login or it silently lapses. A no-op with no
    -- ~/.config/ryoku/power.json, and on a desktop or a box without the knobs.
    hl.exec_cmd("command -v ryoku-power >/dev/null 2>&1 && ryoku-power apply")
    -- Device lighting: push the stored look back at the keyboards and mice the
    -- user put under Ryoku's control (Hub > Appearance > Lighting). A no-op, and
    -- no OpenRGB at all, until lighting is on with a device adopted.
    hl.exec_cmd("command -v ryoku-hub >/dev/null 2>&1 && ryoku-hub lighting apply")
    hl.exec_cmd("command -v ryoku-mic >/dev/null 2>&1 && ryoku-mic")
    -- Night light: hyprsunset is a plain process a reboot drops, so restore the
    -- warm screen when the user left it on. A no-op when it was off or absent.
    hl.exec_cmd("command -v ryoku-cmd-nightlight >/dev/null 2>&1 && ryoku-cmd-nightlight restore")
    -- Booted into a btrfs snapshot from the Limine menu: offer the one-click
    -- restore. limine-snapper-sync ships this as an XDG autostart entry, which
    -- Hyprland never runs (no autostart manager), so start it here; on a normal
    -- boot it detects no snapshot and exits silently.
    hl.exec_cmd("command -v limine-snapper-restore >/dev/null 2>&1 && limine-snapper-restore --notify")
    -- AI UI translation: seed ~/.config/ryoku/i18n-llm.json (empty key) so the
    -- file exists for the user to paste an API key into; idempotent, a no-op if
    -- present. The Hub's Language > "Generate with AI" then reads it.
    hl.exec_cmd("command -v ryoku-i18n >/dev/null 2>&1 && ryoku-i18n ensure >/dev/null 2>&1")
    -- Welcome tour and first-boot keyboard hint: the tour once per tour
    -- version, the hint exactly once ever. The behaviour lives in
    -- ryoku-session-intro so every compositor's autostart calls the same
    -- script. Bump the tour version in the helper when the tour changes
    -- materially (beta 18 = 18).
    hl.exec_cmd("command -v ryoku-session-intro >/dev/null 2>&1 && ryoku-session-intro welcome")
    hl.exec_cmd("command -v ryoku-session-intro >/dev/null 2>&1 && ryoku-session-intro keys-hint")
end)
