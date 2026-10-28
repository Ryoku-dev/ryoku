#include "cavafeed.hpp"

#include <QByteArray>
#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QStandardPaths>

#include <algorithm>
#include <cmath>

namespace {
constexpr int InitialRestartDelayMs = 1200;
constexpr int MaximumRestartDelayMs = 30000;
constexpr int StopGraceMs = 500;
constexpr int SilenceDelayMs = 1000;
constexpr qreal PeakTarget = 0.6;
constexpr qreal PeakReleaseMs = 8000.0;
constexpr qreal QuietBoostMax = 8.0;
constexpr qreal LoudCutMin = 0.5;
}

CavaFeed::CavaFeed(QObject* parent)
    : QObject(parent) {
    m_levels.resize(m_bars);
    std::fill(m_levels.begin(), m_levels.end(), 0.0);
    m_clock.start();

    QString runtimeDir = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (runtimeDir.isEmpty())
        runtimeDir = QDir::tempPath();
    m_configPath = QDir(runtimeDir).filePath(
        QStringLiteral("ryoku-cava-%1-%2.conf")
            .arg(QCoreApplication::applicationPid())
            .arg(reinterpret_cast<quintptr>(this), 0, 16));

    m_restartTimer.setSingleShot(true);
    m_stopTimer.setSingleShot(true);
    m_staleTimer.setSingleShot(true);
    m_silenceTimer.setSingleShot(true);

    connect(&m_restartTimer, &QTimer::timeout, this, &CavaFeed::startProcess);
    connect(&m_stopTimer, &QTimer::timeout, this, [this]() {
        if (m_process.state() != QProcess::NotRunning)
            m_process.kill();
    });
    connect(&m_staleTimer, &QTimer::timeout, this, [this]() {
        if (m_active)
            publishFlat();
    });
    connect(&m_silenceTimer, &QTimer::timeout, this, [this]() {
        if (m_active)
            setSilent(true);
    });
    m_process.setStandardErrorFile(QProcess::nullDevice());
    connect(&m_process, &QProcess::started, this, [this]() {
        m_acceptOutput = true;
        setRunning(true);
        m_process.closeWriteChannel();
    });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &CavaFeed::readOutput);
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int, QProcess::ExitStatus) { handleFinished(); });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error != QProcess::FailedToStart)
            return;
        setRunning(false);
        if (m_active && !m_intentionalStop)
            scheduleRestart();
    });
}

CavaFeed::~CavaFeed() {
    m_active = false;
    m_restartTimer.stop();
    m_stopTimer.stop();
    disconnect(&m_process, nullptr, this, nullptr);
    if (m_process.state() != QProcess::NotRunning) {
        m_process.terminate();
        if (!m_process.waitForFinished(StopGraceMs)) {
            m_process.kill();
            m_process.waitForFinished(StopGraceMs);
        }
    }
    QFile::remove(m_configPath);
}

void CavaFeed::setActive(bool active) {
    if (m_active == active)
        return;
    m_active = active;
    emit activeChanged();
    resetAnalysis();

    if (m_active) {
        m_restartDelayMs = InitialRestartDelayMs;
        startProcess();
    } else {
        m_restartTimer.stop();
        m_restartAfterStop = false;
        stopProcess();
    }
}

void CavaFeed::setBars(int bars) {
    const int bounded = std::clamp(bars, 1, MaxBars);
    if (m_bars == bounded)
        return;
    m_bars = bounded;
    m_levels.resize(m_bars);
    std::fill(m_levels.begin(), m_levels.end(), 0.0);
    m_frameBytes = 0;
    m_energy = 0.0;
    emit barsChanged();
    emit levelsChanged();
    emit energyChanged();
    restartProcess();
}

void CavaFeed::setFramerate(int framerate) {
    const int bounded = std::clamp(framerate, 1, 144);
    if (m_framerate == bounded)
        return;
    m_framerate = bounded;
    emit framerateChanged();
    restartProcess();
}

void CavaFeed::setMethod(const QString& method) {
    const QString bounded = method == QStringLiteral("pulse") ? method : QStringLiteral("pipewire");
    if (m_method == bounded)
        return;
    m_method = bounded;
    emit methodChanged();
    restartProcess();
}

void CavaFeed::setSource(const QString& source) {
    QString bounded = source.trimmed();
    bounded.replace('\n', ' ');
    bounded.replace('\r', ' ');
    if (bounded.isEmpty())
        bounded = QStringLiteral("auto");
    if (m_source == bounded)
        return;
    m_source = bounded;
    emit sourceChanged();
    restartProcess();
}

void CavaFeed::setCaptureActive(bool active) {
    if (m_captureActive == active)
        return;
    m_captureActive = active;
    emit captureActiveChanged();
    restartProcess();
}

void CavaFeed::setAutosens(bool autosens) {
    if (m_autosens == autosens)
        return;
    m_autosens = autosens;
    emit autosensChanged();
    restartProcess();
}

void CavaFeed::setSensitivity(int sensitivity) {
    const int bounded = std::max(sensitivity, 0);
    if (m_sensitivity == bounded)
        return;
    m_sensitivity = bounded;
    emit sensitivityChanged();
    restartProcess();
}

void CavaFeed::setNoiseReduction(int reduction) {
    const int bounded = std::clamp(reduction, 0, 100);
    if (m_noiseReduction == bounded)
        return;
    m_noiseReduction = bounded;
    emit noiseReductionChanged();
    restartProcess();
}

void CavaFeed::setPeakFollow(bool follow) {
    if (m_peakFollow == follow)
        return;
    m_peakFollow = follow;
    m_peak = 0.0;
    m_lastFrameMs = -1;
    emit peakFollowChanged();
}

void CavaFeed::setStaleMs(int staleMs) {
    const int bounded = std::max(staleMs, 1);
    if (m_staleMs == bounded)
        return;
    m_staleMs = bounded;
    emit staleMsChanged();
    if (m_active && m_lastFrameMs >= 0)
        m_staleTimer.start(m_staleMs);
}

void CavaFeed::setNoiseFloor(qreal floor) {
    const qreal bounded = std::clamp(floor, qreal(0.0), qreal(1.0));
    if (qFuzzyCompare(m_noiseFloor + 1.0, bounded + 1.0))
        return;
    m_noiseFloor = bounded;
    emit noiseFloorChanged();
}

void CavaFeed::restartProcess() {
    if (!m_active)
        return;
    if (m_process.state() == QProcess::NotRunning) {
        startProcess();
        return;
    }
    m_restartTimer.stop();
    m_acceptOutput = false;
    m_restartAfterStop = true;
    m_intentionalStop = true;
    m_process.terminate();
    m_stopTimer.start(StopGraceMs);
}

void CavaFeed::startProcess() {
    if (!m_active || m_process.state() != QProcess::NotRunning)
        return;

    const QString executable = QStandardPaths::findExecutable(QStringLiteral("cava"));
    if (executable.isEmpty())
        return;
    if (!writeConfig()) {
        scheduleRestart();
        return;
    }

    m_intentionalStop = false;
    m_acceptOutput = false;
    m_frameBytes = 0;
    m_peak = 0.0;
    m_lastFrameMs = -1;
    m_process.setProcessChannelMode(QProcess::SeparateChannels);
    m_process.start(executable, { QStringLiteral("-p"), m_configPath }, QIODevice::ReadOnly);
}

void CavaFeed::stopProcess() {
    m_staleTimer.stop();
    m_silenceTimer.stop();
    m_acceptOutput = false;
    if (m_process.state() == QProcess::NotRunning) {
        setRunning(false);
        return;
    }
    m_intentionalStop = true;
    m_process.terminate();
    m_stopTimer.start(StopGraceMs);
}

void CavaFeed::scheduleRestart() {
    if (!m_active || QStandardPaths::findExecutable(QStringLiteral("cava")).isEmpty())
        return;
    if (!m_restartTimer.isActive())
        m_restartTimer.start(m_restartDelayMs);
    m_restartDelayMs = std::min(m_restartDelayMs * 2, MaximumRestartDelayMs);
}

void CavaFeed::handleFinished() {
    m_stopTimer.stop();
    m_acceptOutput = false;
    setRunning(false);
    m_frameBytes = 0;

    if (!m_active) {
        m_intentionalStop = false;
        return;
    }
    if (m_restartAfterStop) {
        m_restartAfterStop = false;
        m_intentionalStop = false;
        startProcess();
        return;
    }
    if (m_intentionalStop) {
        m_intentionalStop = false;
        return;
    }
    scheduleRestart();
}

void CavaFeed::readOutput() {
    // cava has parsed its config by the time it writes a frame, and never reads
    // it again. Dropping it here keeps a shell that exits without unwinding
    // (a restart, a crash) from leaving one file per feed in the runtime dir.
    if (m_configOnDisk) {
        QFile::remove(m_configPath);
        m_configOnDisk = false;
    }
    if (!m_acceptOutput) {
        std::array<char, MaxBars * 2> discard{};
        while (m_process.bytesAvailable() > 0)
            m_process.read(discard.data(), std::min<qint64>(discard.size(), m_process.bytesAvailable()));
        return;
    }
    const qsizetype frameSize = static_cast<qsizetype>(m_bars) * 2;
    while (m_process.bytesAvailable() > 0) {
        const qsizetype remaining = frameSize - m_frameBytes;
        const qint64 count = m_process.read(
            reinterpret_cast<char*>(m_binaryFrame.data()) + m_frameBytes, static_cast<qint64>(remaining));
        if (count <= 0)
            break;
        m_frameBytes += static_cast<qsizetype>(count);
        if (m_frameBytes == frameSize) {
            processFrame();
            m_frameBytes = 0;
        }
    }
}

void CavaFeed::processFrame() {
    const qint64 now = m_clock.elapsed();
    const qint64 elapsed = m_lastFrameMs >= 0 ? std::min(now - m_lastFrameMs, qint64(1000)) : 0;
    qreal frameMax = 0.0;

    for (int i = 0; i < m_bars; ++i) {
        const int byte = i * 2;
        const quint16 sample = static_cast<quint16>(m_binaryFrame[byte]) |
                               (static_cast<quint16>(m_binaryFrame[byte + 1]) << 8);
        const qreal value = static_cast<qreal>(sample) / 65535.0;
        m_frameLevels[i] = value;
        frameMax = std::max(frameMax, value);
    }

    qreal gain = 1.0;
    if (m_peakFollow) {
        m_peak = std::max(frameMax, m_peak * std::exp(-static_cast<qreal>(elapsed) / PeakReleaseMs));
        if (m_peak > 0.0)
            gain = std::clamp(PeakTarget / m_peak, LoudCutMin, QuietBoostMax);
    }

    const qreal floor = m_noiseFloor;
    qreal sum = 0.0;
    bool levelsDiffer = false;
    for (int i = 0; i < m_bars; ++i) {
        qreal value = std::min(qreal(1.0), m_frameLevels[i] * gain);
        if (value < floor)
            value = 0.0;
        m_frameLevels[i] = value;
        levelsDiffer = levelsDiffer || m_levels[i] != value;
        sum += value;
    }
    if (levelsDiffer) {
        std::copy_n(m_frameLevels.begin(), m_bars, m_levels.begin());
        emit levelsChanged();
    }

    m_lastFrameMs = now;
    const qreal nextEnergy = sum / static_cast<qreal>(m_bars);
    const bool energyChanged = !qFuzzyCompare(m_energy + 1.0, nextEnergy + 1.0);
    m_energy = nextEnergy;
    if (energyChanged)
        emit this->energyChanged();
    emit frame();

    if (sum > 0.0) {
        m_silenceTimer.stop();
        setSilent(false);
    } else if (!m_silent && !m_silenceTimer.isActive()) {
        m_silenceTimer.start(SilenceDelayMs);
    }

    m_staleTimer.start(m_staleMs);
    m_restartDelayMs = InitialRestartDelayMs;
}

void CavaFeed::publishFlat() {
    bool changed = false;
    for (qreal& level : m_levels) {
        if (level != 0.0) {
            level = 0.0;
            changed = true;
        }
    }
    if (changed)
        emit levelsChanged();
    if (m_energy != 0.0) {
        m_energy = 0.0;
        emit energyChanged();
    }
}

void CavaFeed::resetAnalysis() {
    m_staleTimer.stop();
    m_silenceTimer.stop();
    m_frameBytes = 0;
    m_peak = 0.0;
    m_lastFrameMs = -1;
    publishFlat();
    setSilent(true);
}

bool CavaFeed::writeConfig() {
    QFile config(m_configPath);
    if (!config.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;

    QByteArray contents;
    contents.reserve(384 + m_source.size());
    contents += "[general]\nframerate = ";
    contents += QByteArray::number(m_framerate);
    contents += "\nbars = ";
    contents += QByteArray::number(m_bars);
    contents += "\nautosens = ";
    contents += m_autosens ? "1" : "0";
    contents += "\nsensitivity = ";
    contents += QByteArray::number(m_sensitivity);
    contents += "\n\n[input]\nmethod = ";
    contents += m_method.toUtf8();
    contents += "\nsource = ";
    contents += m_source.toUtf8();
    contents += "\nactive = ";
    contents += m_captureActive ? "1" : "0";
    contents += "\n\n[output]\nmethod = raw\nraw_target = /dev/stdout\ndata_format = binary\nbit_format = 16bit\nchannels = mono\nmono_option = average\n\n[smoothing]\nnoise_reduction = ";
    contents += QByteArray::number(m_noiseReduction);
    contents += '\n';

    m_configOnDisk = true;
    return config.write(contents) == contents.size() && config.flush();
}

void CavaFeed::setRunning(bool running) {
    if (m_running == running)
        return;
    m_running = running;
    emit runningChanged();
}

void CavaFeed::setSilent(bool silent) {
    if (m_silent == silent)
        return;
    m_silent = silent;
    emit silentChanged();
}
