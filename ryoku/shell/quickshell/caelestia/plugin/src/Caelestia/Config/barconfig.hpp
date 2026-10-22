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

class BarScrollActions : public settings::ObjectNode {
    CONFIG_NODE(BarScrollActions, settings::ObjectNode)

    CONFIG_PROPERTY(bool, workspaces, true)
    CONFIG_PROPERTY(bool, volume, true)
    CONFIG_PROPERTY(bool, brightness, true)
};

class BarPopouts : public settings::ObjectNode {
    CONFIG_NODE(BarPopouts, settings::ObjectNode)

    CONFIG_PROPERTY(bool, greeter, false)
    CONFIG_PROPERTY(bool, tray, true)
    CONFIG_PROPERTY(bool, statusIcons, true)
    CONFIG_PROPERTY(bool, clock, false)
};

class BarWorkspaces : public settings::ObjectNode {
    CONFIG_NODE(BarWorkspaces, settings::ObjectNode)

    CONFIG_PROPERTY(int, shown, 5)
    CONFIG_PROPERTY(bool, activeIndicator, true)
    CONFIG_PROPERTY(bool, occupiedBg, false)
    CONFIG_PROPERTY(bool, showUnoccupied, true)
    CONFIG_PROPERTY(bool, showWindows, true)
    CONFIG_PROPERTY(bool, showWindowsOnSpecialWorkspaces, true)
    CONFIG_PROPERTY(int, maxWindowIcons, 5)
    CONFIG_PROPERTY(bool, activeTrail, true)
    CONFIG_PROPERTY(bool, monitorCenter, false)
    CONFIG_PROPERTY(bool, perMonitor, true)
    CONFIG_GLOBAL_PROPERTY(bool, perMonitorWorkspaces, true)
    // Was a boolean called `useIcon`; upstream's name and shape are kept so a
    // shell.json written for either shell means the same thing here.
    CONFIG_ENUM_PROPERTY(BarWorkspaceDisplay, displayType, BarWorkspaceDisplay::Shapes)
    CONFIG_PROPERTY(bool, useIcon, true)
    CONFIG_PROPERTY(QString, label, u" "_s)
    CONFIG_PROPERTY(QString, occupiedLabel, u" 󰮯"_s)
    CONFIG_PROPERTY(QString, activeLabel, u"󰮯 "_s)
    CONFIG_PROPERTY(QString, capitalisation, u"preserve"_s)
    CONFIG_GLOBAL_LIST(IconRuleList, specialWorkspaceIcons, {})
    CONFIG_GLOBAL_PROPERTY(QStringList, ignoredTags,
        DEFAULT_ARG({
            u"hide_in_bar"_s,
            u"xwl_popup"_s,
        }))
    CONFIG_GLOBAL_LIST(IconRuleList, windowIcons,
        DEFAULT_ARG({
            ICON_RULE_REGEX("steam(_app_(default|[0-9]+))?", "", "sports_esports"),
        }))
    CONFIG_GLOBAL_LIST(IconRuleList, wsIcons, {})
};

class BarGreeter : public settings::ObjectNode {
    CONFIG_NODE(BarGreeter, settings::ObjectNode)

    CONFIG_PROPERTY(bool, inverted, false)
    CONFIG_PROPERTY(bool, showOnHover, true)

    CONFIG_PROPERTY(QString, mode, u"timeOfDay"_s)

    CONFIG_PROPERTY(QString, morningGif, u""_s)
    CONFIG_PROPERTY(QString, afternoonGif, u""_s)
    CONFIG_PROPERTY(QString, eveningGif, u""_s)
    CONFIG_PROPERTY(QString, nightGif, u""_s)

    CONFIG_PROPERTY(int, morningStart, 5)
    CONFIG_PROPERTY(int, afternoonStart, 12)
    CONFIG_PROPERTY(int, eveningStart, 17)
    CONFIG_PROPERTY(int, nightStart, 20)

    CONFIG_PROPERTY(QString, morningText, u"Good Morning"_s)
    CONFIG_PROPERTY(QString, afternoonText, u"Good Afternoon"_s)
    CONFIG_PROPERTY(QString, eveningText, u"Good Evening"_s)
    CONFIG_PROPERTY(QString, nightText, u"Good Night"_s)

    CONFIG_PROPERTY(QString, slideshowText, u""_s)
    CONFIG_PROPERTY(QString, slideshowIcon, u"waving_hand"_s)
    CONFIG_PROPERTY(QStringList, slideshowFolders, QStringList())
    CONFIG_PROPERTY(QStringList, slideshowGifs, QStringList())
    CONFIG_PROPERTY(qreal, slideshowInterval, 60.0)
    CONFIG_PROPERTY(bool, slideshowRandom, false)
};

class BarTrayIconSub : public settings::ObjectNode {
    CONFIG_NODE(BarTrayIconSub, settings::ObjectNode)

    CONFIG_PROPERTY(QString, id, {})
    CONFIG_PROPERTY(QString, icon, {})
    CONFIG_PROPERTY(QString, image, {})
};
CONFIG_LIST_TYPE(BarTrayIconSub, BarTrayIconSubList)

class BarTray : public settings::ObjectNode {
    CONFIG_NODE(BarTray, settings::ObjectNode)

    CONFIG_PROPERTY(bool, background, false)
    CONFIG_PROPERTY(bool, recolour, false)
    CONFIG_PROPERTY(bool, compact, true)
    CONFIG_GLOBAL_LIST(BarTrayIconSubList, iconSubs, {})
    CONFIG_GLOBAL_PROPERTY(QStringList, hiddenIcons, QStringList())
};

class BarStatus : public settings::ObjectNode {
    CONFIG_NODE(BarStatus, settings::ObjectNode)

    CONFIG_PROPERTY(bool, showAudio, true)
    CONFIG_PROPERTY(bool, showMicrophone, false)
    CONFIG_PROPERTY(bool, showKbLayout, false)
    CONFIG_PROPERTY(bool, showNetwork, true)
    CONFIG_PROPERTY(bool, showWifi, false)
    CONFIG_PROPERTY(bool, showBluetooth, true)
    CONFIG_PROPERTY(bool, showBattery, true)
    CONFIG_PROPERTY(bool, showPeripheralBattery, false)
    CONFIG_PROPERTY(QStringList, peripheralBatteryExcluded, QStringList())
    CONFIG_PROPERTY(bool, showLockStatus, true)
    CONFIG_PROPERTY(bool, showNotifications, true)
    CONFIG_PROPERTY(bool, showNightLight, true)
};

class BarClock : public settings::ObjectNode {
    CONFIG_NODE(BarClock, settings::ObjectNode)

    CONFIG_PROPERTY(bool, background, false)
    CONFIG_PROPERTY(bool, showDate, false)
    CONFIG_PROPERTY(bool, showIcon, true)
    CONFIG_PROPERTY(bool, centerClock, false)
    CONFIG_PROPERTY(bool, showSeconds, false)
};

class BarDock : public settings::ObjectNode {
    CONFIG_NODE(BarDock, settings::ObjectNode)

    CONFIG_PROPERTY(bool, monitorCenter, true)
    CONFIG_PROPERTY(bool, recolourIcons, false)
    CONFIG_PROPERTY(bool, showBadges, true)
    CONFIG_PROPERTY(int, iconSize, 32)
    CONFIG_PROPERTY(bool, currentDesktopOnly, false)
    CONFIG_ENUM_PROPERTY(DockWindowGrouping, windowGrouping, DockWindowGrouping::Combined)
    CONFIG_PROPERTY(bool, previewOnDesktop, true)
    CONFIG_GLOBAL_PROPERTY(QStringList, pinnedApps, QStringList({ u"firefox"_s, u"org.kde.dolphin"_s }))
};

class BarGithub : public settings::ObjectNode {
    CONFIG_NODE(BarGithub, settings::ObjectNode)

    CONFIG_PROPERTY(bool, background, false)
};

class BarMedia : public settings::ObjectNode {
    CONFIG_NODE(BarMedia, settings::ObjectNode)

    CONFIG_PROPERTY(bool, background, false)
    CONFIG_PROPERTY(bool, showVisualiser, true)
    CONFIG_PROPERTY(int, maxTitleLength, 25)
    CONFIG_PROPERTY(bool, inverted, false)
    CONFIG_PROPERTY(bool, showTitle, true)
    CONFIG_PROPERTY(bool, autoHide, false)
};

class BarPerformance : public settings::ObjectNode {
    CONFIG_NODE(BarPerformance, settings::ObjectNode)

    CONFIG_PROPERTY(bool, showText, true)
};

class BarPreviewScales : public settings::ObjectNode {
    CONFIG_NODE(BarPreviewScales, settings::ObjectNode)

    CONFIG_PROPERTY(qreal, greeter, 0.0)
    CONFIG_PROPERTY(qreal, audio, 0.0)
    CONFIG_PROPERTY(qreal, battery, 0.0)
    CONFIG_PROPERTY(qreal, bluetooth, 0.0)
    CONFIG_PROPERTY(qreal, clock, 0.0)
    CONFIG_PROPERTY(qreal, dock, 0.0)
    CONFIG_PROPERTY(qreal, github, 0.0)
    CONFIG_PROPERTY(qreal, media, 0.0)
    CONFIG_PROPERTY(qreal, kblayout, 0.0)
    CONFIG_PROPERTY(qreal, lockStatus, 0.0)
    CONFIG_PROPERTY(qreal, network, 0.0)
    CONFIG_PROPERTY(qreal, notifications, 0.0)
    CONFIG_PROPERTY(qreal, peripheralBattery, 0.0)
    CONFIG_PROPERTY(qreal, trayMenu, 0.0)
    CONFIG_PROPERTY(qreal, wirelessPassword, 0.0)
};

class BarPreviewFontScales : public settings::ObjectNode {
    CONFIG_NODE(BarPreviewFontScales, settings::ObjectNode)

    CONFIG_PROPERTY(qreal, greeter, 0.0)
    CONFIG_PROPERTY(qreal, audio, 0.0)
    CONFIG_PROPERTY(qreal, battery, 0.0)
    CONFIG_PROPERTY(qreal, bluetooth, 0.0)
    CONFIG_PROPERTY(qreal, clock, 0.0)
    CONFIG_PROPERTY(qreal, dock, 0.0)
    CONFIG_PROPERTY(qreal, github, 0.0)
    CONFIG_PROPERTY(qreal, media, 0.0)
    CONFIG_PROPERTY(qreal, kblayout, 0.0)
    CONFIG_PROPERTY(qreal, lockStatus, 0.0)
    CONFIG_PROPERTY(qreal, network, 0.0)
    CONFIG_PROPERTY(qreal, notifications, 0.0)
    CONFIG_PROPERTY(qreal, peripheralBattery, 0.0)
    CONFIG_PROPERTY(qreal, trayMenu, 0.0)
    CONFIG_PROPERTY(qreal, wirelessPassword, 0.0)
};

class BarConfig : public settings::ObjectNode {
    CONFIG_NODE(BarConfig, settings::ObjectNode)

    CONFIG_PROPERTY(qreal, scale, 1.0)
    CONFIG_PROPERTY(qreal, previewScale, 1.0)
    CONFIG_PROPERTY(bool, previewScaleWithBar, false)
    CONFIG_PROPERTY(bool, perElementPreviewScale, false)
    CONFIG_PROPERTY(bool, perElementFontScale, false)
    CONFIG_PROPERTY(qreal, fontScaleOffset, 0.0)
    CONFIG_PROPERTY(bool, livePreviews, true)
    CONFIG_SUBOBJECT(BarPreviewScales, previewScales)
    CONFIG_SUBOBJECT(BarPreviewFontScales, previewFontScales)
    CONFIG_PROPERTY(bool, persistent, true)
    CONFIG_PROPERTY(bool, dodgeWindows, false)
    CONFIG_PROPERTY(bool, dodgeFocusedOnly, false)
    CONFIG_PROPERTY(bool, showOnHover, true)
    CONFIG_PROPERTY(int, dragThreshold, 20)
    CONFIG_PROPERTY(QString, position, u"left"_s)
    CONFIG_SUBOBJECT(BarScrollActions, scrollActions)
    CONFIG_SUBOBJECT(BarPopouts, popouts)
    CONFIG_SUBOBJECT(BarWorkspaces, workspaces)
    CONFIG_SUBOBJECT(BarGreeter, greeter)
    CONFIG_SUBOBJECT(BarTray, tray)
    CONFIG_SUBOBJECT(BarStatus, status)
    // The status area as an ordered list: which icons are there and in what order,
    // which is upstream's shape for it and what the settings editor reads. An `id`
    // names one of the icons the bar knows how to draw; `enabled` is its switch.
    CONFIG_LIST(EntryList, statusIcons,
        DEFAULT_ARG({
            LIST_ENTRY(lockStatus, true),
            LIST_ENTRY(microphone, false),
            LIST_ENTRY(kbLayout, false),
            LIST_ENTRY(network, true),
            LIST_ENTRY(ethernet, true),
            LIST_ENTRY(bluetooth, true),
            LIST_ENTRY(audio, true),
            LIST_ENTRY(battery, true),
            LIST_ENTRY(peripheralBattery, false),
            LIST_ENTRY(nightlight, true),
            LIST_ENTRY(notifications, true),
        }))
    CONFIG_SUBOBJECT(BarClock, clock)
    CONFIG_SUBOBJECT(BarDock, dock)
    CONFIG_SUBOBJECT(BarGithub, github)
    CONFIG_SUBOBJECT(BarMedia, media)
    CONFIG_SUBOBJECT(BarPerformance, performance)
    CONFIG_PROPERTY(QVariantList, entries,
        DEFAULT_ARG({
            vmap({ { u"id"_s, u"logo"_s }, { u"enabled"_s, true }, { u"zone"_s, u"left"_s } }),
            vmap({ { u"id"_s, u"workspaces"_s }, { u"enabled"_s, true }, { u"zone"_s, u"left"_s } }),
            vmap({ { u"id"_s, u"greeter"_s }, { u"enabled"_s, true }, { u"zone"_s, u"left"_s } }),
            vmap({ { u"id"_s, u"dock"_s }, { u"enabled"_s, true }, { u"zone"_s, u"middle"_s } }),
            vmap({ { u"id"_s, u"tray"_s }, { u"enabled"_s, true }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"updateIndicator"_s }, { u"enabled"_s, true }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"github"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"media"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"clock"_s }, { u"enabled"_s, true }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"statusIcons"_s }, { u"enabled"_s, true }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"kbLayoutIndicator"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"notificationsIndicator"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfCpu"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfMemory"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfStorage"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfNetwork"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfGpu"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"perfBattery"_s }, { u"enabled"_s, false }, { u"zone"_s, u"right"_s } }),
            vmap({ { u"id"_s, u"power"_s }, { u"enabled"_s, true }, { u"zone"_s, u"right"_s } }),
        }))
    CONFIG_PROPERTY(QStringList, excludedScreens, QStringList())
};

} // namespace caelestia::config
