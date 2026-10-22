// SPDX-License-Identifier: GPL-3.0-only
#pragma once

#include <qdatetime.h>
#include <qjsonarray.h>
#include <qjsondocument.h>
#include <qjsonobject.h>
#include <qnetworkaccessmanager.h>
#include <qnetworkreply.h>
#include <qpointer.h>
#include <qqmlintegration.h>
#include <qtimer.h>
#include <qvariant.h>

#include "service.hpp"

namespace caelestia::services {

class Weather : public Service {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(QString city READ city NOTIFY cityChanged)
    Q_PROPERTY(QString loc READ loc NOTIFY locChanged)
    Q_PROPERTY(QString temp READ temp NOTIFY weatherChanged)
    Q_PROPERTY(qreal tempC READ tempC NOTIFY weatherChanged)
    Q_PROPERTY(QString feelsLike READ feelsLike NOTIFY weatherChanged)
    Q_PROPERTY(qreal feelsLikeC READ feelsLikeC NOTIFY weatherChanged)
    Q_PROPERTY(QString description READ description NOTIFY weatherChanged)
    Q_PROPERTY(QString icon READ icon NOTIFY weatherChanged)
    Q_PROPERTY(int weatherCode READ weatherCode NOTIFY weatherChanged)
    Q_PROPERTY(int humidity READ humidity NOTIFY weatherChanged)
    Q_PROPERTY(qreal windSpeed READ windSpeed NOTIFY weatherChanged)
    Q_PROPERTY(QString sunrise READ sunrise NOTIFY weatherChanged)
    Q_PROPERTY(QString sunset READ sunset NOTIFY weatherChanged)
    Q_PROPERTY(QString maxTemp READ maxTemp NOTIFY weatherChanged)
    Q_PROPERTY(qreal maxTempC READ maxTempC NOTIFY weatherChanged)
    Q_PROPERTY(QString minTemp READ minTemp NOTIFY weatherChanged)
    Q_PROPERTY(qreal minTempC READ minTempC NOTIFY weatherChanged)
    Q_PROPERTY(QVariantList forecast READ forecast NOTIFY forecastChanged)
    Q_PROPERTY(QVariantList hourlyForecast READ hourlyForecast NOTIFY hourlyForecastChanged)
    Q_PROPERTY(bool hasWeather READ hasWeather NOTIFY weatherChanged)
    Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)

public:
    explicit Weather(QObject* parent = nullptr);
    ~Weather() override;

    [[nodiscard]] QString city() const;
    [[nodiscard]] QString loc() const;
    [[nodiscard]] QString temp() const;
    [[nodiscard]] qreal tempC() const;
    [[nodiscard]] QString feelsLike() const;
    [[nodiscard]] qreal feelsLikeC() const;
    [[nodiscard]] QString description() const;
    [[nodiscard]] QString icon() const;
    [[nodiscard]] int weatherCode() const;
    [[nodiscard]] int humidity() const;
    [[nodiscard]] qreal windSpeed() const;
    [[nodiscard]] QString sunrise() const;
    [[nodiscard]] QString sunset() const;
    [[nodiscard]] QString maxTemp() const;
    [[nodiscard]] qreal maxTempC() const;
    [[nodiscard]] QString minTemp() const;
    [[nodiscard]] qreal minTempC() const;
    [[nodiscard]] QVariantList forecast() const;
    [[nodiscard]] QVariantList hourlyForecast() const;
    [[nodiscard]] bool hasWeather() const;
    [[nodiscard]] bool loading() const;

    [[nodiscard]] Q_INVOKABLE QString formatTemp(const QVariant& tempVal = QVariant()) const;
    [[nodiscard]] static QString getWeatherIcon(int code, bool isDay = true);
    [[nodiscard]] static QString getWeatherCondition(int code);

    Q_INVOKABLE void setLocation(const QString& coords, const QString& cityName = QString());
    Q_INVOKABLE void setCity(const QString& cityName);
    Q_INVOKABLE void reload();
    Q_INVOKABLE void refresh();

signals:
    void cityChanged();
    void locChanged();
    void weatherChanged();
    void forecastChanged();
    void hourlyForecastChanged();
    void loadingChanged();

protected:
    void start() override;
    void stop() override;

private:
    void fetchLocation();
    void fetchCityFromCoords(const QString& coords);
    void fetchWeatherData();
    void parseWeatherJson(const QJsonObject& json);
    void loadCache();
    void saveCache(const QJsonObject& json);
    void loadCachedCity();
    void saveCachedCity(const QString& coords, const QString& cityName);

    [[nodiscard]] static QString cacheFilePath();
    [[nodiscard]] static QString citiesCacheFilePath();

    QNetworkAccessManager* m_nam = nullptr;
    QPointer<QNetworkReply> m_locReply;
    QPointer<QNetworkReply> m_cityReply;
    QPointer<QNetworkReply> m_weatherReply;
    QTimer* m_refreshTimer = nullptr;

    QString m_city;
    QString m_loc;
    qreal m_tempC = 0.0;
    qreal m_feelsLikeC = 0.0;
    QString m_description;
    QString m_icon;
    int m_weatherCode = 0;
    int m_humidity = 0;
    qreal m_windSpeed = 0.0;
    QString m_sunrise;
    QString m_sunset;
    qreal m_maxTempC = 0.0;
    qreal m_minTempC = 0.0;
    QVariantList m_forecast;
    QVariantList m_hourlyForecast;
    bool m_hasWeather = false;
    bool m_loading = false;
    bool m_isDay = true;
};

} // namespace caelestia::services
