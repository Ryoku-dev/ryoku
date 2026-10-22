#pragma once

#include <qstring.h>
#include <qstringlist.h>
#include <qvariant.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"
#include "enums.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

class LauncherUseFuzzy : public settings::ObjectNode {
    CONFIG_NODE(LauncherUseFuzzy, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(bool, apps, false)
    CONFIG_GLOBAL_PROPERTY(bool, actions, false)
    CONFIG_GLOBAL_PROPERTY(bool, schemes, false)
    CONFIG_GLOBAL_PROPERTY(bool, variants, false)
    CONFIG_GLOBAL_PROPERTY(bool, wallpapers, false)
    CONFIG_GLOBAL_PROPERTY(bool, emoji, false)
    CONFIG_GLOBAL_PROPERTY(bool, clipboard, false)
};

class LauncherAction : public settings::ObjectNode {
    CONFIG_NODE(LauncherAction, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(QString, name, {})
    CONFIG_PROPERTY(QString, icon, {})
    CONFIG_PROPERTY(QString, description, {})
    CONFIG_PROPERTY(QStringList, command, {})
    CONFIG_PROPERTY(bool, dangerous, false)
};
CONFIG_LIST_TYPE(LauncherAction, LauncherActionList)

class LauncherConfig : public settings::ObjectNode {
    CONFIG_NODE(LauncherConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(bool, showOnHover, false)
    CONFIG_PROPERTY(int, maxShown, 7)
    CONFIG_PROPERTY(int, maxWallpapers, 9)
    CONFIG_PROPERTY(int, clipboardMaxEntries, 20)
    CONFIG_GLOBAL_PROPERTY(QString, specialPrefix, u"@"_s)
    CONFIG_GLOBAL_PROPERTY(QString, actionPrefix, u">"_s)
    CONFIG_GLOBAL_PROPERTY(bool, enableDangerousActions, false)
    CONFIG_PROPERTY(int, dragThreshold, 50)
    CONFIG_PROPERTY(bool, showPowerMenu, true)
    CONFIG_PROPERTY(bool, showBrowseOnEmpty, true)
    CONFIG_ENUM_PROPERTY(LauncherBrowseLayout, browseLayout, LauncherBrowseLayout::Default)
    CONFIG_ENUM_PROPERTY(LauncherNameOverflow, nameOverflow, LauncherNameOverflow::Tooltip)
    CONFIG_PROPERTY(int, hoverThickness, 10)
    CONFIG_PROPERTY(int, hoverWidth, 50)
    CONFIG_GLOBAL_PROPERTY(bool, vimKeybinds, false)
    CONFIG_GLOBAL_PROPERTY(bool, confirmClearClipboard, true)
    CONFIG_GLOBAL_PROPERTY(QStringList, favouriteApps, QStringList({ u"firefox"_s, u"org.kde.dolphin"_s }))
    CONFIG_GLOBAL_PROPERTY(QStringList, hiddenApps, QStringList())
    CONFIG_GLOBAL_PROPERTY(QStringList, favouriteEmojis, QStringList())
    CONFIG_GLOBAL_PROPERTY(QStringList, favouriteClips, QStringList())
    CONFIG_SUBOBJECT(LauncherUseFuzzy, useFuzzy)
    CONFIG_GLOBAL_LIST(LauncherActionList, actions,
        DEFAULT_ARG({
            vmap({
                { u"name"_s, u"Wallpaper"_s },
                { u"icon"_s, u"image"_s },
                { u"description"_s, u"Change the current wallpaper"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"wallpaper"_s } },
            }),
            vmap({
                { u"name"_s, u"Scheme"_s },
                { u"icon"_s, u"palette"_s },
                { u"description"_s, u"Change the current color scheme"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"scheme"_s } },
            }),
            vmap({
                { u"name"_s, u"Variant"_s },
                { u"icon"_s, u"colors"_s },
                { u"description"_s, u"Change the current scheme variant"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"variant"_s } },
            }),
            vmap({
                { u"name"_s, u"Random"_s },
                { u"icon"_s, u"casino"_s },
                { u"description"_s, u"Switch to a random wallpaper"_s },
                { u"command"_s, QStringList{ u"ryogami"_s, u"wallpaper"_s, u"random"_s } },
            }),
            vmap({
                { u"name"_s, u"Light"_s },
                { u"icon"_s, u"light_mode"_s },
                { u"description"_s, u"Change the scheme to light mode"_s },
                { u"command"_s, QStringList{ u"setMode"_s, u"light"_s } },
            }),
            vmap({
                { u"name"_s, u"Dark"_s },
                { u"icon"_s, u"dark_mode"_s },
                { u"description"_s, u"Change the scheme to dark mode"_s },
                { u"command"_s, QStringList{ u"setMode"_s, u"dark"_s } },
            }),
            vmap({
                { u"name"_s, u"Animations"_s },
                { u"icon"_s, u"animation"_s },
                { u"description"_s, u"Switch your animation style"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"animations"_s } },
            }),
            vmap({
                { u"name"_s, u"Lock"_s },
                { u"icon"_s, u"lock"_s },
                { u"description"_s, u"Lock the current session"_s },
                { u"command"_s, QStringList{ u"ryoku-shell"_s, u"lock"_s } },
            }),
            vmap({
                { u"name"_s, u"Logout"_s },
                { u"icon"_s, u"exit_to_app"_s },
                { u"description"_s, u"Log out of the current session"_s },
                { u"command"_s, QStringList{ u"logout"_s } },
                { u"dangerous"_s, true },
            }),
            vmap({
                { u"name"_s, u"Sleep"_s },
                { u"icon"_s, u"bedtime"_s },
                { u"description"_s, u"Suspend then hibernate"_s },
                { u"command"_s, QStringList{ u"suspendThenHibernate"_s } },
            }),
            vmap({
                { u"name"_s, u"Shutdown"_s },
                { u"icon"_s, u"power_settings_new"_s },
                { u"description"_s, u"Shutdown the system"_s },
                { u"command"_s, QStringList{ u"poweroff"_s } },
                { u"dangerous"_s, true },
            }),
            vmap({
                { u"name"_s, u"Reboot"_s },
                { u"icon"_s, u"cached"_s },
                { u"description"_s, u"Reboot the system"_s },
                { u"command"_s, QStringList{ u"reboot"_s } },
                { u"dangerous"_s, true },
            }),
            vmap({
                { u"name"_s, u"Calculator"_s },
                { u"icon"_s, u"calculate"_s },
                { u"description"_s, u"Do simple math equations (powered by Qalc)"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"calc"_s } },
            }),
            vmap({
                { u"name"_s, u"Emoji"_s },
                { u"icon"_s, u"emoji_emotions"_s },
                { u"description"_s, u"Pick an emoji to copy"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"emoji"_s } },
            }),
            vmap({
                { u"name"_s, u"Clipboard"_s },
                { u"icon"_s, u"content_paste"_s },
                { u"description"_s, u"View clipboard history"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"clipboard"_s } },
            }),
            vmap({
                { u"name"_s, u"Windows"_s },
                { u"icon"_s, u"apps"_s },
                { u"description"_s, u"Switch to another window"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"windows"_s } },
                { u"enabled"_s, true },
            }),
            vmap({
                { u"name"_s, u"Keybinds"_s },
                { u"icon"_s, u"keyboard"_s },
                { u"description"_s, u"View all keybinds"_s },
                { u"command"_s, QStringList{ u"autocomplete"_s, u"keybinds"_s } },
            }),
            vmap({
                { u"name"_s, u"Settings"_s },
                { u"icon"_s, u"settings"_s },
                { u"description"_s, u"Configure the shell"_s },
                { u"command"_s, QStringList{ u"ryoku-shell"_s, u"hub"_s, u"open"_s } },
            }),

        }))
};

} // namespace caelestia::config
