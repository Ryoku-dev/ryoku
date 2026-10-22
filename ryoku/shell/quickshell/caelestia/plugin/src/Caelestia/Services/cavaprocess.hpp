#pragma once

#include <qprocess.h>
#include <qqmlintegration.h>
#include <qtemporaryfile.h>
#include <qvector.h>

#include "service.hpp"

namespace caelestia::services {

// The visualiser source when the plugin is built without libcava (it is not in
// the Arch repos): the same CavaProvider type, fed by the cava program from
// extra/cava, which captures PipeWire itself and prints one line of bar
// heights per frame.
class CavaProvider : public Service {
    Q_OBJECT
    QML_ELEMENT

    Q_PROPERTY(int bars READ bars WRITE setBars NOTIFY barsChanged)

    Q_PROPERTY(QVector<double> values READ values NOTIFY valuesChanged)

public:
    explicit CavaProvider(QObject* parent = nullptr);
    ~CavaProvider() override;

    [[nodiscard]] int bars() const;
    void setBars(int bars);

    [[nodiscard]] QVector<double> values() const;

signals:
    void barsChanged();
    void valuesChanged();

private:
    void start() override;
    void stop() override;
    void readFrames();

    QProcess m_process;
    QTemporaryFile m_config;
    bool m_running = false;
    int m_bars = 0;
    QVector<double> m_values;
};

} // namespace caelestia::services
