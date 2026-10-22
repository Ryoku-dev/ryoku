// SPDX-License-Identifier: GPL-3.0-only
#include "weather.hpp"

#include <qdir.h>
#include <qfile.h>
#include <qfileinfo.h>
#include <qloggingcategory.h>
#include <qstandardpaths.h>
#include <qurlquery.h>

#include <cmath>

#include "../Config/rootnodes.hpp"
#include "../Config/serviceconfig.hpp"

Q_LOGGING_CATEGORY(lcWeather, "caelestia.services.weather", QtInfoMsg)

namespace caelestia::services {

Weather::Weather(QObject* parent)
    : Service(parent)
    , m_nam(new QNetworkAccessManager(this)) {
    loadCache();
    loadCachedCity();

    const auto* cfg = config::ConfigSingleton::instance();
    auto* const svcCfg = cfg ? cfg->services() : nullptr;
    if (svcCfg) {
        connect(svcCfg, &config::ServiceConfig::weatherLocationChanged, this, [this]() {
            const auto* c = config::ConfigSingleton::instance();
            const auto* s = c ? c->services() : nullptr;
            const QString cfgLoc = s ? s->weatherLocation() : QString();
            if (cfgLoc.isEmpty()) {
                m_loc.clear();
                m_city.clear();
                emit locChanged();
                emit cityChanged();
                fetchLocation();
            } else {
                m_loc = cfgLoc;
                emit locChanged();
                loadCachedCity();
                if (m_city.isEmpty()) {
                    fetchCityFromCoords(m_loc);
                }
                fetchWeatherData();
            }
        });
        connect(svcCfg, &config::ServiceConfig::weatherUnitsChanged, this, [this]() {
            emit weatherChanged();
            emit forecastChanged();
        });
        connect(svcCfg, &config::ServiceConfig::clockFormatChanged, this, [this]() {
            emit weatherChanged();
        });
    }
}

Weather::~Weather() = default;

QString Weather::city() const {
    return m_city;
}

QString Weather::loc() const {
    return m_loc;
}

QString Weather::temp() const {
    if (!m_hasWeather) {
        return QStringLiteral("--°C");
    }
    return formatTemp(m_tempC);
}

qreal Weather::tempC() const {
    return m_tempC;
}

QString Weather::feelsLike() const {
    if (!m_hasWeather) {
        return QStringLiteral("--°C");
    }
    return formatTemp(m_feelsLikeC);
}

qreal Weather::feelsLikeC() const {
    return m_feelsLikeC;
}

QString Weather::description() const {
    if (m_description.isEmpty()) {
        return tr("No weather");
    }
    return m_description;
}

QString Weather::icon() const {
    if (m_icon.isEmpty()) {
        return QStringLiteral("cloud");
    }
    return m_icon;
}

int Weather::weatherCode() const {
    return m_weatherCode;
}

int Weather::humidity() const {
    return m_humidity;
}

qreal Weather::windSpeed() const {
    return m_windSpeed;
}

QString Weather::sunrise() const {
    return m_sunrise.isEmpty() ? QStringLiteral("--:--") : m_sunrise;
}

QString Weather::sunset() const {
    return m_sunset.isEmpty() ? QStringLiteral("--:--") : m_sunset;
}

QString Weather::maxTemp() const {
    if (!m_hasWeather) {
        return QStringLiteral("--°C");
    }
    return formatTemp(m_maxTempC);
}

qreal Weather::maxTempC() const {
    return m_maxTempC;
}

QString Weather::minTemp() const {
    if (!m_hasWeather) {
        return QStringLiteral("--°C");
    }
    return formatTemp(m_minTempC);
}

qreal Weather::minTempC() const {
    return m_minTempC;
}

QVariantList Weather::forecast() const {
    return m_forecast;
}

QVariantList Weather::hourlyForecast() const {
    return m_hourlyForecast;
}

bool Weather::hasWeather() const {
    return m_hasWeather;
}

bool Weather::loading() const {
    return m_loading;
}

QString Weather::formatTemp(const QVariant& tempVal) const {
    const auto* cfg = config::ConfigSingleton::instance();
    const auto* svcCfg = cfg ? cfg->services() : nullptr;
    const auto unit = svcCfg ? svcCfg->weatherUnit() : config::TemperatureUnit::Celsius;

    const auto placeholder = [unit]() -> QString {
        switch (unit) {
        case config::TemperatureUnit::Fahrenheit:
            return QStringLiteral("--°F");
        case config::TemperatureUnit::Kelvin:
            return QStringLiteral("-- K");
        default:
            return QStringLiteral("--°C");
        }
    };

    if (!tempVal.isValid() || tempVal.isNull()) {
        return placeholder();
    }

    bool ok = false;
    const double val = tempVal.toDouble(&ok);
    if (!ok || std::isnan(val)) {
        return placeholder();
    }

    switch (unit) {
    case config::TemperatureUnit::Fahrenheit:
        return QStringLiteral("%1°F").arg(static_cast<int>(std::round(val * 9.0 / 5.0 + 32.0)));
    case config::TemperatureUnit::Kelvin:
        return QStringLiteral("%1 K").arg(static_cast<int>(std::round(val + 273.15)));
    default:
        return QStringLiteral("%1°C").arg(static_cast<int>(std::round(val)));
    }
}

QString Weather::getWeatherIcon(int code, bool isDay) {
    switch (code) {
    case 0:
    case 1:
        return isDay ? QStringLiteral("clear_day") : QStringLiteral("clear_night");
    case 2:
        return isDay ? QStringLiteral("partly_cloudy_day") : QStringLiteral("partly_cloudy_night");
    case 3:
        return QStringLiteral("cloud");
    case 45:
    case 48:
        return QStringLiteral("foggy");
    case 51:
    case 53:
    case 55:
    case 56:
    case 57:
    case 61:
    case 63:
    case 65:
    case 66:
    case 67:
    case 80:
    case 81:
    case 82:
        return QStringLiteral("rainy");
    case 71:
    case 73:
    case 77:
    case 85:
        return QStringLiteral("cloudy_snowing");
    case 75:
    case 86:
        return QStringLiteral("snowing_heavy");
    case 95:
    case 96:
    case 99:
        return QStringLiteral("thunderstorm");
    default:
        return QStringLiteral("air");
    }
}

QString Weather::getWeatherCondition(int code) {
    switch (code) {
    case 0:
    case 1:
        return tr("Clear");
    case 2:
        return tr("Partly cloudy");
    case 3:
        return tr("Overcast");
    case 45:
    case 48:
        return tr("Fog");
    case 51:
    case 53:
    case 55:
        return tr("Drizzle");
    case 56:
    case 57:
        return tr("Freezing drizzle");
    case 61:
    case 66:
    case 80:
        return tr("Light rain");
    case 63:
    case 81:
        return tr("Rain");
    case 65:
    case 67:
    case 82:
        return tr("Heavy rain");
    case 71:
        return tr("Light snow");
    case 73:
    case 77:
        return tr("Snow");
    case 75:
        return tr("Heavy snow");
    case 85:
        return tr("Light snow showers");
    case 86:
        return tr("Heavy snow showers");
    case 95:
        return tr("Thunderstorm");
    case 96:
    case 99:
        return tr("Thunderstorm with hail");
    default:
        return tr("Unknown");
    }
}

void Weather::setLocation(const QString& coords, const QString& cityName) {
    m_loc = coords;
    emit locChanged();
    if (!cityName.isEmpty()) {
        m_city = cityName;
        saveCachedCity(coords, cityName);
        emit cityChanged();
    } else if (!coords.isEmpty()) {
        loadCachedCity();
        if (m_city.isEmpty()) {
            fetchCityFromCoords(coords);
        }
    } else {
        m_city.clear();
        emit cityChanged();
    }
    fetchWeatherData();
}

void Weather::setCity(const QString& cityName) {
    if (m_city != cityName) {
        m_city = cityName;
        if (!m_loc.isEmpty()) {
            saveCachedCity(m_loc, cityName);
        }
        emit cityChanged();
    }
}

void Weather::reload() {
    fetchWeatherData();
}

void Weather::refresh() {
    reload();
}

void Weather::start() {
    if (!m_hasWeather) {
        loadCache();
    }
    if (!m_refreshTimer) {
        m_refreshTimer = new QTimer(this);
        m_refreshTimer->setInterval(900000);
        connect(m_refreshTimer, &QTimer::timeout, this, &Weather::fetchWeatherData);
    }
    m_refreshTimer->start();
    fetchWeatherData();
}

void Weather::stop() {
    if (m_refreshTimer) {
        m_refreshTimer->stop();
    }
    if (m_locReply) {
        m_locReply->abort();
        m_locReply->deleteLater();
    }
    if (m_cityReply) {
        m_cityReply->abort();
        m_cityReply->deleteLater();
    }
    if (m_weatherReply) {
        m_weatherReply->abort();
        m_weatherReply->deleteLater();
    }
    if (m_loading) {
        m_loading = false;
        emit loadingChanged();
    }
}

void Weather::fetchLocation() {
    if (m_locReply) {
        return;
    }

    QUrl url(QStringLiteral("https://ipinfo.io/json"));
    QNetworkRequest req(url);
    req.setHeader(QNetworkRequest::UserAgentHeader, QStringLiteral("caelestia-shell/1.0"));
    m_locReply = m_nam->get(req);

    connect(m_locReply, &QNetworkReply::finished, this, [this]() {
        if (!m_locReply)
            return;
        if (m_locReply->error() == QNetworkReply::NoError) {
            const auto data = m_locReply->readAll();
            const auto doc = QJsonDocument::fromJson(data);
            if (doc.isObject()) {
                const auto obj = doc.object();
                const QString locStr = obj.value(QStringLiteral("loc")).toString();
                const QString cityStr = obj.value(QStringLiteral("city")).toString();
                if (!locStr.isEmpty()) {
                    m_loc = locStr;
                    emit locChanged();
                    if (!cityStr.isEmpty()) {
                        m_city = cityStr;
                        saveCachedCity(m_loc, m_city);
                        emit cityChanged();
                    }
                    fetchWeatherData();
                }
            }
        } else if (m_locReply->error() != QNetworkReply::OperationCanceledError) {
            qCWarning(lcWeather) << "Failed to fetch auto location from ipinfo:" << m_locReply->errorString();
        }
        m_locReply->deleteLater();
    });
}

void Weather::fetchCityFromCoords(const QString& coords) {
    if (coords.isEmpty() || !coords.contains(QLatin1Char(',')))
        return;
    if (m_cityReply) {
        return;
    }

    const auto parts = coords.split(QLatin1Char(','));
    if (parts.size() < 2)
        return;
    const QString lat = parts.at(0).trimmed();
    const QString lon = parts.at(1).trimmed();

    QUrl url(QStringLiteral(
        "https://api.bigdatacloud.net/data/reverse-geocode-client?latitude=%1&longitude=%2&localityLanguage=en")
            .arg(lat, lon));
    QNetworkRequest req(url);
    req.setHeader(QNetworkRequest::UserAgentHeader, QStringLiteral("caelestia-shell/1.0"));
    m_cityReply = m_nam->get(req);

    connect(m_cityReply, &QNetworkReply::finished, this, [this, coords]() {
        if (!m_cityReply)
            return;
        if (m_cityReply->error() == QNetworkReply::NoError) {
            const auto data = m_cityReply->readAll();
            const auto doc = QJsonDocument::fromJson(data);
            if (doc.isObject()) {
                const auto obj = doc.object();
                QString cityName = obj.value(QStringLiteral("city")).toString();
                if (cityName.isEmpty()) {
                    cityName = obj.value(QStringLiteral("locality")).toString();
                }
                if (cityName.isEmpty()) {
                    cityName = obj.value(QStringLiteral("principalSubdivision")).toString();
                }
                if (!cityName.isEmpty() && m_loc == coords) {
                    m_city = cityName;
                    saveCachedCity(coords, cityName);
                    emit cityChanged();
                }
            }
        } else if (m_cityReply->error() != QNetworkReply::OperationCanceledError) {
            qCWarning(lcWeather) << "Failed to fetch city from coords:" << m_cityReply->errorString();
        }
        m_cityReply->deleteLater();
    });
}

void Weather::fetchWeatherData() {
    const auto* cfg = config::ConfigSingleton::instance();
    const auto* svcCfg = cfg ? cfg->services() : nullptr;
    QString configLoc = svcCfg ? svcCfg->weatherLocation() : QString();

    if (configLoc.isEmpty() && m_loc.isEmpty()) {
        fetchLocation();
        return;
    }

    if (!configLoc.isEmpty() && configLoc.contains(QLatin1Char(','))) {
        m_loc = configLoc;
    }

    if (m_loc.isEmpty()) {
        fetchLocation();
        return;
    }

    const auto parts = m_loc.split(QLatin1Char(','));
    if (parts.size() < 2) {
        return;
    }
    const QString lat = parts.at(0).trimmed();
    const QString lon = parts.at(1).trimmed();

    if (m_city.isEmpty()) {
        loadCachedCity();
        if (m_city.isEmpty()) {
            fetchCityFromCoords(m_loc);
        }
    }

    if (m_weatherReply) {
        return;
    }

    QUrl url(QStringLiteral("https://api.open-meteo.com/v1/forecast"));
    QUrlQuery query;
    query.addQueryItem(QStringLiteral("latitude"), lat);
    query.addQueryItem(QStringLiteral("longitude"), lon);
    query.addQueryItem(
        QStringLiteral("hourly"), QStringLiteral("weather_code,temperature_2m,precipitation_probability"));
    query.addQueryItem(
        QStringLiteral("daily"), QStringLiteral("weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset"));
    query.addQueryItem(QStringLiteral("current"),
        QStringLiteral("temperature_2m,relative_humidity_2m,apparent_temperature,is_day,weather_code,wind_speed_10m"));
    query.addQueryItem(QStringLiteral("timezone"), QStringLiteral("auto"));
    query.addQueryItem(QStringLiteral("forecast_days"), QStringLiteral("7"));
    url.setQuery(query);

    m_loading = true;
    emit loadingChanged();

    QNetworkRequest request(url);
    request.setHeader(QNetworkRequest::UserAgentHeader, QStringLiteral("caelestia-shell/1.0"));
    m_weatherReply = m_nam->get(request);

    connect(m_weatherReply, &QNetworkReply::finished, this, [this]() {
        if (!m_weatherReply)
            return;
        m_loading = false;
        emit loadingChanged();

        if (m_weatherReply->error() == QNetworkReply::NoError) {
            const auto data = m_weatherReply->readAll();
            const auto doc = QJsonDocument::fromJson(data);
            if (doc.isObject()) {
                const auto obj = doc.object();
                parseWeatherJson(obj);
                saveCache(obj);
            }
        } else if (m_weatherReply->error() != QNetworkReply::OperationCanceledError) {
            qCWarning(lcWeather) << "Failed to fetch weather data:" << m_weatherReply->errorString();
        }
        m_weatherReply->deleteLater();
    });
}

void Weather::parseWeatherJson(const QJsonObject& json) {
    if (!json.contains(QStringLiteral("current")) || !json.contains(QStringLiteral("daily"))) {
        return;
    }

    const auto current = json.value(QStringLiteral("current")).toObject();
    const auto daily = json.value(QStringLiteral("daily")).toObject();
    const auto hourly = json.value(QStringLiteral("hourly")).toObject();

    m_weatherCode = current.value(QStringLiteral("weather_code")).toInt();
    m_tempC = current.value(QStringLiteral("temperature_2m")).toDouble();
    m_feelsLikeC = current.value(QStringLiteral("apparent_temperature")).toDouble();
    m_humidity = current.value(QStringLiteral("relative_humidity_2m")).toInt();
    m_windSpeed = current.value(QStringLiteral("wind_speed_10m")).toDouble();
    m_isDay = current.value(QStringLiteral("is_day")).toInt(1) != 0;

    m_description = getWeatherCondition(m_weatherCode);
    m_icon = getWeatherIcon(m_weatherCode, m_isDay);

    const auto* cfg = config::ConfigSingleton::instance();
    const auto* svcCfg = cfg ? cfg->services() : nullptr;
    const bool use12h = svcCfg ? svcCfg->twelveHourClock() : false;
    const QString timeFormat = use12h ? QStringLiteral("h:mm AP") : QStringLiteral("h:mm");

    const auto dailyMax = daily.value(QStringLiteral("temperature_2m_max")).toArray();
    const auto dailyMin = daily.value(QStringLiteral("temperature_2m_min")).toArray();
    const auto dailySunrise = daily.value(QStringLiteral("sunrise")).toArray();
    const auto dailySunset = daily.value(QStringLiteral("sunset")).toArray();
    const auto dailyCodes = daily.value(QStringLiteral("weather_code")).toArray();
    const auto dailyTimes = daily.value(QStringLiteral("time")).toArray();

    if (!dailyMax.isEmpty())
        m_maxTempC = dailyMax.at(0).toDouble();
    if (!dailyMin.isEmpty())
        m_minTempC = dailyMin.at(0).toDouble();

    if (!dailySunrise.isEmpty()) {
        const QDateTime dt = QDateTime::fromString(dailySunrise.at(0).toString(), Qt::ISODate);
        m_sunrise = dt.isValid() ? dt.time().toString(timeFormat) : QStringLiteral("--:--");
    }
    if (!dailySunset.isEmpty()) {
        const QDateTime dt = QDateTime::fromString(dailySunset.at(0).toString(), Qt::ISODate);
        m_sunset = dt.isValid() ? dt.time().toString(timeFormat) : QStringLiteral("--:--");
    }

    const qsizetype dailyCount =
        qMin(dailyTimes.size(), qMin(dailyMax.size(), qMin(dailyMin.size(), dailyCodes.size())));
    QVariantList forecastList;
    for (qsizetype i = 0; i < dailyCount; ++i) {
        const double maxC = dailyMax.at(i).toDouble();
        const double minC = dailyMin.at(i).toDouble();
        const int code = dailyCodes.at(i).toInt();
        QString dateStr = dailyTimes.at(i).toString();
        dateStr.replace(QLatin1Char('-'), QLatin1Char('/'));

        QVariantMap item;
        item[QStringLiteral("date")] = dateStr;
        item[QStringLiteral("maxTempC")] = maxC;
        item[QStringLiteral("minTempC")] = minC;
        item[QStringLiteral("maxTempF")] = static_cast<int>(std::round(maxC * 9.0 / 5.0 + 32.0));
        item[QStringLiteral("minTempF")] = static_cast<int>(std::round(minC * 9.0 / 5.0 + 32.0));
        item[QStringLiteral("weatherCode")] = code;
        item[QStringLiteral("icon")] = getWeatherIcon(code, true);
        forecastList.append(item);
    }
    m_forecast = forecastList;

    QVariantList hourlyList;
    const auto hourlyTimes = hourly.value(QStringLiteral("time")).toArray();
    const auto hourlyTemps = hourly.value(QStringLiteral("temperature_2m")).toArray();
    const auto hourlyPrecip = hourly.value(QStringLiteral("precipitation_probability")).toArray();
    const auto hourlyCodes = hourly.value(QStringLiteral("weather_code")).toArray();

    const qsizetype hourlyCount =
        qMin(hourlyTimes.size(), qMin(hourlyTemps.size(), qMin(hourlyPrecip.size(), hourlyCodes.size())));

    const QDateTime now = QDateTime::currentDateTime();
    for (qsizetype i = 0; i < hourlyCount; ++i) {
        const QString timeStr = hourlyTimes.at(i).toString();
        const QDateTime dt = QDateTime::fromString(timeStr, Qt::ISODate);
        if (dt.isValid() && dt < now) {
            continue;
        }

        const int code = hourlyCodes.at(i).toInt();
        QVariantMap item;
        item[QStringLiteral("timestamp")] = timeStr;
        item[QStringLiteral("hour")] = dt.isValid() ? dt.time().hour() : 0;
        item[QStringLiteral("tempC")] = static_cast<int>(std::round(hourlyTemps.at(i).toDouble()));
        item[QStringLiteral("precipChance")] = hourlyPrecip.at(i).toInt();
        item[QStringLiteral("weatherCode")] = code;
        item[QStringLiteral("icon")] = getWeatherIcon(code, true);
        hourlyList.append(item);
    }
    m_hourlyForecast = hourlyList;

    m_hasWeather = true;
    emit weatherChanged();
    emit forecastChanged();
    emit hourlyForecastChanged();
}

QString Weather::cacheFilePath() {
    const QString cacheDir =
        QStandardPaths::writableLocation(QStandardPaths::CacheLocation) + QStringLiteral("/caelestia");
    QDir().mkpath(cacheDir);
    return cacheDir + QStringLiteral("/weather_cache.json");
}

QString Weather::citiesCacheFilePath() {
    const QString cacheDir =
        QStandardPaths::writableLocation(QStandardPaths::CacheLocation) + QStringLiteral("/caelestia");
    QDir().mkpath(cacheDir);
    return cacheDir + QStringLiteral("/cities.json");
}

void Weather::loadCache() {
    QFile f(cacheFilePath());
    if (!f.open(QIODevice::ReadOnly)) {
        return;
    }
    const auto doc = QJsonDocument::fromJson(f.readAll());
    if (doc.isObject()) {
        parseWeatherJson(doc.object());
    }
}

void Weather::saveCache(const QJsonObject& json) {
    QFile f(cacheFilePath());
    if (f.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        f.write(QJsonDocument(json).toJson(QJsonDocument::Compact));
    }
}

void Weather::loadCachedCity() {
    if (m_loc.isEmpty())
        return;
    QFile f(citiesCacheFilePath());
    if (!f.open(QIODevice::ReadOnly))
        return;
    const auto doc = QJsonDocument::fromJson(f.readAll());
    if (doc.isObject()) {
        const auto obj = doc.object();
        if (obj.contains(m_loc)) {
            m_city = obj.value(m_loc).toString();
            emit cityChanged();
        }
    }
}

void Weather::saveCachedCity(const QString& coords, const QString& cityName) {
    if (coords.isEmpty() || cityName.isEmpty())
        return;
    QJsonObject obj;
    QFile f(citiesCacheFilePath());
    if (f.open(QIODevice::ReadOnly)) {
        const auto doc = QJsonDocument::fromJson(f.readAll());
        if (doc.isObject()) {
            obj = doc.object();
        }
        f.close();
    }
    obj[coords] = cityName;
    if (f.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        f.write(QJsonDocument(obj).toJson(QJsonDocument::Compact));
    }
}

} // namespace caelestia::services
