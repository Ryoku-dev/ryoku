#include "serviceconfig.hpp"

#include <qlocale.h>

namespace caelestia::config {

namespace {

TemperatureUnit::Enum resolveTemperatureUnit(TemperatureUnit::Enum unit) {
    if (unit != TemperatureUnit::Auto)
        return unit;

    const auto system = QLocale().measurementSystem();
    return system == QLocale::ImperialUSSystem || system == QLocale::ImperialUKSystem ? TemperatureUnit::Fahrenheit
                                                                                      : TemperatureUnit::Celsius;
}

} // namespace

bool ServiceConfig::twelveHourClock() const {
    const auto format = clockFormat();
    if (format == ClockFormat::Auto)
        return QLocale().timeFormat(QLocale::ShortFormat).toLower().contains(u"a"_s);

    return format == ClockFormat::TwelveHour;
}

TemperatureUnit::Enum ServiceConfig::weatherUnit() const {
    return resolveTemperatureUnit(weatherUnits());
}

TemperatureUnit::Enum ServiceConfig::sensorUnit() const {
    return resolveTemperatureUnit(sensorUnits());
}

} // namespace caelestia::config
