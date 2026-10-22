#include "cavaprocess.hpp"

#include <qloggingcategory.h>

#include <algorithm>
#include <cmath>

namespace caelestia::services {

Q_LOGGING_CATEGORY(lcCavaProcess, "caelestia.services.cava", QtWarningMsg)

namespace {

constexpr int kMaxRange = 1000;

}

CavaProvider::CavaProvider(QObject* parent)
    : Service(parent) {
    m_process.setProgram(QStringLiteral("cava"));
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &CavaProvider::readFrames);
    connect(&m_process, &QProcess::errorOccurred, this, [](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            qCWarning(lcCavaProcess) << "cava is not installed, the visualiser stays flat";
    });
}

CavaProvider::~CavaProvider() {
    m_process.kill();
    m_process.waitForFinished(500);
}

int CavaProvider::bars() const {
    return m_bars;
}

void CavaProvider::setBars(int bars) {
    bars = std::max(0, bars);
    if (m_bars == bars)
        return;
    m_bars = bars;
    m_values.fill(0.0, bars);
    emit barsChanged();
    emit valuesChanged();
    if (m_running) {
        stop();
        start();
    }
}

QVector<double> CavaProvider::values() const {
    return m_values;
}

void CavaProvider::start() {
    m_running = true;
    if (m_bars == 0 || !m_config.open())
        return;
    m_config.resize(0);
    m_config.write(QStringLiteral("[general]\nframerate = 60\nbars = %1\nautosens = 1\n"
                                  "lower_cutoff_freq = 50\nhigher_cutoff_freq = 10000\n\n"
                                  "[input]\nmethod = pipewire\nsource = auto\n\n"
                                  "[output]\nmethod = raw\nraw_target = /dev/stdout\ndata_format = ascii\n"
                                  "ascii_max_range = %2\nchannels = mono\nmono_option = average\n\n"
                                  "[smoothing]\nnoise_reduction = 85\n")
            .arg(m_bars)
            .arg(kMaxRange)
            .toUtf8());
    m_config.flush();
    m_process.setArguments({ QStringLiteral("-p"), m_config.fileName() });
    m_process.start();
}

void CavaProvider::stop() {
    m_running = false;
    m_process.kill();
    m_process.waitForFinished(500);
    if (std::any_of(m_values.cbegin(), m_values.cend(), [](double v) {
            return v != 0.0;
        })) {
        m_values.fill(0.0);
        emit valuesChanged();
    }
}

void CavaProvider::readFrames() {
    // only the newest complete frame matters; older ones are already late
    QByteArray last;
    while (m_process.canReadLine())
        last = m_process.readLine();
    if (last.isEmpty())
        return;

    const auto parts = last.trimmed().split(';');
    QVector<double> raw(m_bars, 0.0);
    for (int i = 0; i < m_bars && i < parts.size(); ++i)
        raw[i] = std::clamp(parts[i].toDouble() / kMaxRange, 0.0, 1.0);

    // the same spread the libcava provider applies: a peak falls off over its neighbours
    const double inv = 1.0 / 1.5;
    QVector<double> out(m_bars, 0.0);
    double carry = 0.0;
    for (int i = 0; i < m_bars; ++i) {
        carry = std::max(raw[i], carry * inv);
        out[i] = carry;
    }
    carry = 0.0;
    for (int i = m_bars - 1; i >= 0; --i) {
        carry = std::max(raw[i], carry * inv);
        out[i] = std::max(out[i], carry);
    }

    constexpr double epsilon = 0.0005;
    for (int i = 0; i < m_bars; ++i) {
        if (std::abs(out[i] - m_values[i]) > epsilon) {
            m_values = out;
            emit valuesChanged();
            return;
        }
    }
}

} // namespace caelestia::services
