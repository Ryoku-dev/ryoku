#pragma once

#include <QFileInfo>
#include <QStandardPaths>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

class GeneralApps : public settings::ObjectNode {
    CONFIG_NODE(GeneralApps, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(QStringList, terminal, { u"foot"_s })
    CONFIG_GLOBAL_PROPERTY(QStringList, audio, { u"xdg-open"_s })
    CONFIG_GLOBAL_PROPERTY(QStringList, playback, { u"xdg-open"_s })
    CONFIG_GLOBAL_PROPERTY(QStringList, explorer, { u"xdg-open"_s })
};

class GeneralBatteryWarnLevel : public settings::ObjectNode {
    CONFIG_NODE(GeneralBatteryWarnLevel, settings::ObjectNode)

    CONFIG_PROPERTY(int, level, -1)
    CONFIG_PROPERTY(QString, title, {})
    CONFIG_PROPERTY(QString, message, {})
    CONFIG_PROPERTY(QString, icon, {})
    CONFIG_PROPERTY(bool, critical, false)
};
CONFIG_LIST_TYPE(GeneralBatteryWarnLevel, GeneralBatteryWarnList)

class GeneralBattery : public settings::ObjectNode {
    CONFIG_NODE(GeneralBattery, settings::ObjectNode)

    CONFIG_GLOBAL_LIST(GeneralBatteryWarnList, warnLevels,
        DEFAULT_ARG({
            vmap({
                { u"level"_s, 20 },
                { u"title"_s, u"Low battery"_s },
                { u"message"_s, u"You might want to plug in a charger"_s },
                { u"icon"_s, u"battery_android_frame_2"_s },
            }),
            vmap({
                { u"level"_s, 10 },
                { u"title"_s, u"Did you see the previous message?"_s },
                { u"message"_s, u"You should probably plug in a charger <b>now</b>"_s },
                { u"icon"_s, u"battery_android_frame_1"_s },
            }),
            vmap({
                { u"level"_s, 5 },
                { u"title"_s, u"Critical battery level"_s },
                { u"message"_s, u"PLUG THE CHARGER RIGHT NOW!!"_s },
                { u"icon"_s, u"battery_android_alert"_s },
                { u"critical"_s, true },
            }),
        }))
};

class GeneralConfig : public settings::ObjectNode {
    CONFIG_NODE(GeneralConfig, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(QString, logo, QString())
    CONFIG_PROPERTY(bool, showOverFullscreen, false)
    CONFIG_PROPERTY(qreal, mediaGifSpeedAdjustment, 300)
    CONFIG_PROPERTY(qreal, sessionGifSpeed, 0.7)
    CONFIG_PROPERTY(QString, language, QStringLiteral("system"))
    CONFIG_PROPERTY(bool, debugLogs, false)
    CONFIG_PROPERTY(bool, checkUpdates, true)
    CONFIG_PROPERTY(bool, magicLampEnabled, true)
    CONFIG_PROPERTY(bool, caelestiaMode, false)
    CONFIG_PROPERTY(bool, krohnkiteEnabled, false)
    CONFIG_PROPERTY(QString, krohnkiteLastLayout, QStringLiteral("BTree"))
    CONFIG_SUBOBJECT(GeneralApps, apps)
    CONFIG_SUBOBJECT(GeneralBattery, battery)
};

} // namespace caelestia::config
