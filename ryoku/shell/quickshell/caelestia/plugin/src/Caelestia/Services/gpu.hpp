#pragma once

#include <qprocess.h>
#include <qqmlintegration.h>

#include "tickingservice.hpp"

namespace caelestia::services {

class Gpu : public TickingService {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

public:
    enum Type {
        Auto,
        None,
        Nvidia,
        Generic,
    };
    Q_ENUM(Type)

private:
    Q_PROPERTY(Type type READ type NOTIFY typeChanged)
    Q_PROPERTY(Type userType READ userType NOTIFY userTypeChanged)
    Q_PROPERTY(Type autoType READ autoType NOTIFY autoTypeChanged)
    Q_PROPERTY(QString name READ name NOTIFY nameChanged)
    Q_PROPERTY(qreal percentage READ percentage NOTIFY percentageChanged)
    Q_PROPERTY(qreal temperature READ temperature NOTIFY temperatureChanged)

public:
    explicit Gpu(QObject* parent = nullptr);

    [[nodiscard]] Type type() const;
    [[nodiscard]] Type userType() const;
    [[nodiscard]] Type autoType() const;
    [[nodiscard]] QString name() const;
    [[nodiscard]] qreal percentage() const;
    [[nodiscard]] qreal temperature() const;

signals:
    void typeChanged();
    void userTypeChanged();
    void autoTypeChanged();
    void nameChanged();
    void percentageChanged();
    void temperatureChanged();

protected:
    void tick() override;

private:
    void detectGpu();
    void finishLspciProbe(const QByteArray& out);
    void probeNvidiaCapability();
    void tryNameSource(int index);
    void finishNameSource(int index, QString name);
    void readGenericUsage();
    void startNvidiaUsage();
    void readGpuTemperature();
    void resetReadings();

    void runProcess(const QString& program, const QStringList& args, std::function<void(const QByteArray&)> callback);

    void setUserType(Type value);
    void setAutoType(Type value);
    void setName(QString value);

    [[nodiscard]] static Type parseType(const QString& s);

    Type m_userType = Auto;
    Type m_autoType = None;
    QString m_name;
    qreal m_percentage = 0.0;
    qreal m_temperature = 0.0;

    QStringList m_busyFiles;

    QString m_nvidiaPciPath;

    int m_nvidiaFailures = 0;

    bool m_detecting = false;
    bool m_nvidiaQuerying = false;
};

} // namespace caelestia::services
