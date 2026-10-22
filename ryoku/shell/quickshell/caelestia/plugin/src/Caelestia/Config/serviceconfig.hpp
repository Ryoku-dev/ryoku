#pragma once

#include <qlocale.h>
#include <qstring.h>
#include <qstringlist.h>
#include <qvariant.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"
#include "enums.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

class PlayerAlias : public settings::ObjectNode {
    CONFIG_NODE(PlayerAlias, settings::ObjectNode)

    CONFIG_PROPERTY(QString, from, {})
    CONFIG_PROPERTY(QString, to, {})
};
CONFIG_LIST_TYPE(PlayerAlias, PlayerAliasList)

class ServiceConfig : public settings::ObjectNode {
    CONFIG_NODE(ServiceConfig, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(QString, weatherLocation, QString())
    CONFIG_GLOBAL_ENUM_PROPERTY(TemperatureUnit, weatherUnits, TemperatureUnit::Auto)
    CONFIG_GLOBAL_ENUM_PROPERTY(TemperatureUnit, sensorUnits, TemperatureUnit::Celsius)
    CONFIG_GLOBAL_ENUM_PROPERTY(DataUnit, dataUnits, DataUnit::Binary)
    CONFIG_GLOBAL_PROPERTY(bool, useFahrenheit,
        QLocale().measurementSystem() == QLocale::ImperialUSSystem ||
            QLocale().measurementSystem() == QLocale::ImperialUKSystem)
    CONFIG_GLOBAL_PROPERTY(bool, useFahrenheitPerformance, false)
    CONFIG_GLOBAL_PROPERTY(bool, useTwelveHourClock, false)
    CONFIG_GLOBAL_ENUM_PROPERTY(ClockFormat, clockFormat, ClockFormat::Auto)

public:
    Q_PROPERTY(bool twelveHourClock READ twelveHourClock NOTIFY clockFormatChanged)
    Q_PROPERTY(caelestia::config::TemperatureUnit::Enum weatherUnit READ weatherUnit NOTIFY weatherUnitsChanged)
    Q_PROPERTY(caelestia::config::TemperatureUnit::Enum sensorUnit READ sensorUnit NOTIFY sensorUnitsChanged)

    [[nodiscard]] bool twelveHourClock() const;
    [[nodiscard]] TemperatureUnit::Enum weatherUnit() const;
    [[nodiscard]] TemperatureUnit::Enum sensorUnit() const;

private:
    CONFIG_GLOBAL_PROPERTY(QString, gpuType, QString())
    CONFIG_GLOBAL_PROPERTY(int, visualiserBars, 60)
    // What the audio visualisers (and the beat tracker sharing their capture
    // stream) react to: the default output's monitor, or the default input.
    CONFIG_GLOBAL_ENUM_PROPERTY(VisualiserInput, visualiserInput, VisualiserInput::Output)
    CONFIG_GLOBAL_PROPERTY(qreal, audioIncrement, 0.1)
    CONFIG_GLOBAL_PROPERTY(qreal, brightnessIncrement, 0.1)
    CONFIG_GLOBAL_PROPERTY(qreal, maxVolume, 1.0)
    CONFIG_GLOBAL_PROPERTY(bool, smartScheme, true)

    CONFIG_GLOBAL_PROPERTY(bool, useSystemd, false)
    CONFIG_GLOBAL_PROPERTY(QString, wallhavenApiKey, QString())

    CONFIG_GLOBAL_PROPERTY(bool, autoSchemeEnabled, false)
    CONFIG_GLOBAL_PROPERTY(QString, autoSchemeMode, u"solar"_s)
    // "HH:MM", local time. Also used as the fallback when solar times cannot be
    // computed (no location set, or polar day/night).
    CONFIG_GLOBAL_PROPERTY(QString, autoSchemeLightTime, u"07:00"_s)
    CONFIG_GLOBAL_PROPERTY(QString, autoSchemeDarkTime, u"19:00"_s)
    CONFIG_GLOBAL_PROPERTY(QString, defaultPlayer, u"Spotify"_s)
    CONFIG_GLOBAL_LIST(PlayerAliasList, playerAliases,
        { vmap({ { u"from"_s, u"com.github.th_ch.youtube_music"_s }, { u"to"_s, u"YT Music"_s } }) })
    CONFIG_GLOBAL_PROPERTY(QString, lyricsBackend, u"Auto"_s)
    CONFIG_GLOBAL_PROPERTY(QStringList, bluetoothAutoReconnectDevices, QStringList())

    CONFIG_GLOBAL_PROPERTY(QString, hotspotSsid, QString())
    CONFIG_GLOBAL_PROPERTY(QString, hotspotPassword, QString())

    CONFIG_GLOBAL_PROPERTY(bool, quickShareAutoStart, false)

    CONFIG_GLOBAL_PROPERTY(bool, arpcEnabled, false)
    CONFIG_GLOBAL_PROPERTY(QString, arpcClientId, u"1126685412586733678"_s)
    CONFIG_GLOBAL_PROPERTY(QString, arpcAppName, u"Caelestia Shell"_s)
    CONFIG_GLOBAL_PROPERTY(QString, arpcDetails, u""_s)
    CONFIG_GLOBAL_PROPERTY(QString, arpcState, u""_s)
    CONFIG_GLOBAL_PROPERTY(QString, arpcLargeImage, u""_s)
    CONFIG_GLOBAL_PROPERTY(QString, arpcSmallImage, u""_s)
    CONFIG_GLOBAL_PROPERTY(bool, arpcSteamAutoDetect, false)
    CONFIG_GLOBAL_PROPERTY(QStringList, arpcSteamBlacklist, QStringList())
    CONFIG_GLOBAL_PROPERTY(QStringList, arpcTargetWindows, QStringList())
    CONFIG_GLOBAL_PROPERTY(QStringList, arpcTargetWindowLabels, QStringList())
    CONFIG_GLOBAL_PROPERTY(bool, arpcCaelestiaInfo, false)
    CONFIG_GLOBAL_PROPERTY(bool, arpcManualOverride, false)
    CONFIG_GLOBAL_PROPERTY(int, arpcIdleTimeout, 0)
};

} // namespace caelestia::config
