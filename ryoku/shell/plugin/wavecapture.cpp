#include "wavecapture.hpp"

#include <QRegularExpression>
#include <QtEndian>

#include <algorithm>
#include <cmath>
#include <utility>

WaveCapture::WaveCapture(QObject* parent) : QObject(parent) {
    m_restart.setSingleShot(true);
    m_restart.setInterval(1000);
    connect(&m_restart, &QTimer::timeout, this, &WaveCapture::startResolution);

    m_stop.setSingleShot(true);
    m_stop.setInterval(500);
    connect(&m_stop, &QTimer::timeout, this, [this] {
        if (m_capture.state() != QProcess::NotRunning)
            m_capture.kill();
    });

    m_stale.setInterval(120);
    connect(&m_stale, &QTimer::timeout, this, [this] {
        if (!m_active || (m_lastFrame.isValid() && m_lastFrame.elapsed() <= 300))
            return;
        setSilent(true);
        publishFlat();
    });

    connect(&m_resolver, &QProcess::finished, this, &WaveCapture::handleResolverExit);
    connect(&m_resolver, &QProcess::errorOccurred, this,
            [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart && m_active) {
            m_resolveStage = ResolveStage::Idle;
            scheduleRestart();
        }
    });
    connect(&m_capture, &QProcess::readyReadStandardOutput, this, &WaveCapture::readPcm);
    connect(&m_capture, &QProcess::readyReadStandardError, &m_capture,
            [this] { m_capture.readAllStandardError(); });
    connect(&m_capture, &QProcess::errorOccurred, this,
            [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart && m_active) {
            setRunning(false);
            scheduleRestart();
        }
    });
    connect(&m_capture, &QProcess::started, this, [this] {
        m_stop.stop();
        setRunning(true);
        m_lastFrame.restart();
        m_lastNonSilent.restart();
        m_stale.start();
    });
    connect(&m_capture, &QProcess::finished, this, [this](int, QProcess::ExitStatus) {
        m_stop.stop();
        setRunning(false);
        m_stale.stop();
        m_pcm.clear();
        if (m_active)
            scheduleRestart();
    });
}

WaveCapture::~WaveCapture() {
    m_active = false;
    stopCapture();
    if (m_resolver.state() != QProcess::NotRunning) {
        m_resolver.kill();
        m_resolver.waitForFinished(200);
    }
    if (m_capture.state() != QProcess::NotRunning) {
        m_capture.kill();
        m_capture.waitForFinished(200);
    }
}

void WaveCapture::setActive(bool active) {
    if (m_active == active)
        return;
    m_active = active;
    if (active)
        startResolution();
    else
        stopCapture();
    emit activeChanged();
}
void WaveCapture::setSource(const QString& source) {
    QString bounded = source.trimmed();
    bounded.replace('\n', ' ');
    bounded.replace('\r', ' ');
    if (bounded == QStringLiteral("auto"))
        bounded.clear();
    if (m_source == bounded)
        return;
    m_source = bounded;
    emit sourceChanged();
    if (m_active) {
        stopCapture();
        scheduleRestart();
    }
}

void WaveCapture::startResolution() {
    if (!m_active || m_resolver.state() != QProcess::NotRunning
        || m_capture.state() != QProcess::NotRunning)
        return;

    m_restart.stop();
    if (m_source.isEmpty()) {
        m_defaultSink.clear();
        m_explicitResolution = false;
        m_resolveStage = ResolveStage::DefaultSink;
        m_resolver.setProgram(QStringLiteral("pactl"));
        m_resolver.setArguments({QStringLiteral("get-default-sink")});
    } else {
        m_defaultSink = m_source;
        m_explicitResolution = true;
        m_resolveStage = ResolveStage::Sources;
        m_resolver.setProgram(QStringLiteral("pactl"));
        m_resolver.setArguments({QStringLiteral("list"), QStringLiteral("short"),
                                 QStringLiteral("sources")});
    }
    m_resolver.start();
}

void WaveCapture::handleResolverExit(int exitCode, QProcess::ExitStatus status) {
    if (!m_active)
        return;

    if (status != QProcess::NormalExit || exitCode != 0) {
        m_resolveStage = ResolveStage::Idle;
        scheduleRestart();
        return;
    }

    if (m_resolveStage == ResolveStage::DefaultSink) {
        m_defaultSink = QString::fromUtf8(m_resolver.readAllStandardOutput()).trimmed();
        if (m_defaultSink.isEmpty()) {
            m_resolveStage = ResolveStage::Idle;
            scheduleRestart();
            return;
        }
        m_resolveStage = ResolveStage::Sources;
        m_resolver.setProgram(QStringLiteral("pactl"));
        m_resolver.setArguments({QStringLiteral("list"), QStringLiteral("short"),
                                 QStringLiteral("sources")});
        m_resolver.start();
        return;
    }

    const QString wanted = m_defaultSink + QStringLiteral(".monitor");
    const QString output = QString::fromUtf8(m_resolver.readAllStandardOutput());
    QString fallback;
    QString target;
    const QStringList lines = output.split(QRegularExpression(QStringLiteral("[\\r\\n]+")),
                                           Qt::SkipEmptyParts);
    for (const QString& line : lines) {
        const QStringList fields = line.split(QRegularExpression(QStringLiteral("\\s+")),
                                              Qt::SkipEmptyParts);
        if (fields.size() < 2 || !fields[1].endsWith(QStringLiteral(".monitor")))
            continue;
        if (fallback.isEmpty())
            fallback = fields[0];
        if (fields[1] == wanted) {
            target = fields[0];
            break;
        }
    }
    if (target.isEmpty() && m_explicitResolution) {
        m_explicitResolution = false;
        m_defaultSink.clear();
        m_resolveStage = ResolveStage::DefaultSink;
        m_resolver.setProgram(QStringLiteral("pactl"));
        m_resolver.setArguments({QStringLiteral("get-default-sink")});
        m_resolver.start();
        return;
    }
    if (target.isEmpty())
        target = fallback;
    m_resolveStage = ResolveStage::Idle;
    if (target.isEmpty()) {
        scheduleRestart();
        return;
    }
    startCapture(target);
}

void WaveCapture::startCapture(const QString& target) {
    if (!m_active || m_capture.state() != QProcess::NotRunning)
        return;

    m_pcm.clear();
    m_pcmOffset = 0;
    m_capture.setProgram(QStringLiteral("pw-record"));
    m_capture.setArguments({QStringLiteral("--target"), target,
                            QStringLiteral("--rate"), QString::number(sampleRate),
                            QStringLiteral("--channels"), QStringLiteral("1"),
                            QStringLiteral("--format"), QStringLiteral("s16"),
                            QStringLiteral("--raw"), QStringLiteral("-")});
    m_capture.start();
}

void WaveCapture::stopCapture() {
    m_restart.stop();
    m_stale.stop();
    m_resolveStage = ResolveStage::Idle;
    if (m_resolver.state() != QProcess::NotRunning)
        m_resolver.kill();
    if (m_capture.state() != QProcess::NotRunning)
        m_capture.terminate();
    if (m_capture.state() != QProcess::NotRunning)
        m_stop.start();
    m_pcm.clear();
    m_pcmOffset = 0;
    if (m_capture.state() == QProcess::NotRunning)
        setRunning(false);
    setSilent(true);
    publishFlat();
}

void WaveCapture::readPcm() {
    if (!m_active)
        return;
    m_pcm.append(m_capture.readAllStandardOutput());
    constexpr qsizetype windowBytes = windowSamples * qsizetype(sizeof(qint16));
    constexpr qsizetype hopBytes = hopSamples * qsizetype(sizeof(qint16));
    while (m_pcm.size() - m_pcmOffset >= windowBytes) {
        processFrame(m_pcm.constData() + m_pcmOffset);
        m_pcmOffset += hopBytes;
    }
    if (m_pcmOffset >= 65536) {
        m_pcm.remove(0, m_pcmOffset);
        m_pcmOffset = 0;
    }
}

void WaveCapture::processFrame(const char* pcm) {
    constexpr int step = std::max(1, windowSamples / outputPoints);
    QList<qreal> next;
    next.reserve(outputPoints);
    int peak = 0;
    for (int i = 0; i < windowSamples && next.size() < outputPoints; i += step) {
        const qint16 sample = qFromLittleEndian<qint16>(
            reinterpret_cast<const uchar*>(pcm + i * int(sizeof(qint16))));
        peak = std::max(peak, std::abs(int(sample)));
        next.append(qreal(sample) / 32768.0);
    }

    m_lastFrame.restart();
    if (peak > 1) {
        m_lastNonSilent.restart();
        setSilent(false);
    } else if (m_lastNonSilent.isValid() && m_lastNonSilent.elapsed() >= 1000) {
        setSilent(true);
        publishFlat();
        return;
    }

    m_samples = std::move(next);
    emit samplesChanged();
    emit frame();
}

void WaveCapture::publishFlat() {
    bool changed = m_samples.size() != outputPoints;
    if (!changed) {
        for (qreal sample : m_samples) {
            if (sample != 0) {
                changed = true;
                break;
            }
        }
    }
    if (!changed)
        return;
    m_samples = QList<qreal>(outputPoints, 0);
    emit samplesChanged();
    emit frame();
}

void WaveCapture::setRunning(bool running) {
    if (m_running == running)
        return;
    m_running = running;
    emit runningChanged();
}

void WaveCapture::setSilent(bool silent) {
    if (m_silent == silent)
        return;
    m_silent = silent;
    emit silentChanged();
}

void WaveCapture::scheduleRestart() {
    if (m_active && !m_restart.isActive())
        m_restart.start();
}
