#pragma once

#include <qstring.h>
#include <qvariant.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

class UtilitiesToasts : public settings::ObjectNode {
    CONFIG_NODE(UtilitiesToasts, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(QString, fullscreen, u"off"_s)
    CONFIG_GLOBAL_PROPERTY(bool, configLoaded, false)
    CONFIG_GLOBAL_PROPERTY(bool, chargingChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, gameModeChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, dndChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, audioOutputChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, audioInputChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, capsLockChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, numLockChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, kbLayoutChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, kbLimit, true)
    CONFIG_GLOBAL_PROPERTY(bool, vpnChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, nowPlaying, false)
    CONFIG_GLOBAL_PROPERTY(bool, clipboardChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, nightLightChanged, true)
    CONFIG_GLOBAL_PROPERTY(bool, transparency, false)
    CONFIG_GLOBAL_PROPERTY(qreal, transparencyBase, 0.85)
};

class UtilitiesVpnProvider : public settings::ObjectNode {
    CONFIG_NODE(UtilitiesVpnProvider, settings::ObjectNode)

    CONFIG_PROPERTY(QString, id, {})
    CONFIG_PROPERTY(QString, name, {})
    CONFIG_PROPERTY(QString, displayName, {})
    CONFIG_PROPERTY(QString, interface, {})
    CONFIG_PROPERTY(QStringList, connectCmd, {})
    CONFIG_PROPERTY(QStringList, disconnectCmd, {})
};
CONFIG_LIST_TYPE(UtilitiesVpnProvider, UtilitiesVpnProviderList)

class UtilitiesVpn : public settings::ObjectNode {
    CONFIG_NODE(UtilitiesVpn, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(bool, enabled, false)
    CONFIG_GLOBAL_LIST(UtilitiesVpnProviderList, provider, {})
    CONFIG_GLOBAL_PROPERTY(QString, selectedProvider, QString())
};

class UtilitiesGameMode : public settings::ObjectNode {
    CONFIG_NODE(UtilitiesGameMode, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(bool, disableHyprlandAnimations, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableHyprlandBlur, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableHyprlandGaps, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableHyprlandShadows, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableShellTransparency, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableWindowTransparency, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableToastTransparency, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableDesktopLyrics, true)
    CONFIG_GLOBAL_PROPERTY(bool, disableVisualizer, true)

    CONFIG_GLOBAL_PROPERTY(bool, autoEnable, true)
    CONFIG_GLOBAL_PROPERTY(QStringList, autoEnableRegexes, QStringList())
};

class UtilitiesConfig : public settings::ObjectNode {
    CONFIG_NODE(UtilitiesConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(bool, showOnHover, true)
    CONFIG_PROPERTY(int, dragThreshold, 50)
    CONFIG_PROPERTY(int, hoverThickness, 10)
    CONFIG_PROPERTY(int, hoverWidth, 50)
    CONFIG_PROPERTY(int, maxToasts, 4)
    CONFIG_SUBOBJECT(UtilitiesToasts, toasts)
    CONFIG_SUBOBJECT(UtilitiesVpn, vpn)
    CONFIG_SUBOBJECT(UtilitiesGameMode, gameMode)
    CONFIG_PROPERTY(bool, showKeepAwake, true)
    CONFIG_PROPERTY(bool, showScreenRecorder, true)
    CONFIG_PROPERTY(bool, showGifRecorder, true)
    CONFIG_PROPERTY(bool, showQuickShare, true)
    CONFIG_PROPERTY(bool, showQuickToggles, true)
    CONFIG_PROPERTY(QVariantList, quickToggles,
        DEFAULT_ARG({
            vmap({ { u"id"_s, u"wifi"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"bluetooth"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"mic"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"settings"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"gameMode"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"colorpicker"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"dnd"_s }, { u"enabled"_s, true } }),
            vmap({ { u"id"_s, u"vpn"_s }, { u"enabled"_s, false } }),
            vmap({ { u"id"_s, u"wallpaper"_s }, { u"enabled"_s, true } }),
        }))
    CONFIG_PROPERTY(bool, quickTogglesCustomOrder, false)
    CONFIG_PROPERTY(int, quickTogglesPerPage, 6)
};

} // namespace caelestia::config
