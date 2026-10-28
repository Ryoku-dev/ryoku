#pragma once

#include <QByteArray>
#include <QElapsedTimer>
#include <QList>
#include <QObject>
#include <QProcess>
#include <QTimer>
#include <qqmlregistration.h>

class WaveCapture : public QObject {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(bool active READ active WRITE setActive NOTIFY activeChanged)
    Q_PROPERTY(QString source READ source WRITE setSource NOTIFY sourceChanged)
    Q_PROPERTY(QList<qreal> samples READ samples NOTIFY samplesChanged)
    Q_PROPERTY(bool running READ running NOTIFY runningChanged)
    Q_PROPERTY(bool silent READ silent NOTIFY silentChanged)

public:
    explicit WaveCapture(QObject* parent = nullptr);
    ~WaveCapture() override;

    bool active() const { return m_active; }
    void setActive(bool active);
    const QString& source() const { return m_source; }
    void setSource(const QString& source);
    const QList<qreal>& samples() const { return m_samples; }
    bool running() const { return m_running; }
    bool silent() const { return m_silent; }

signals:
    void sourceChanged();
    void activeChanged();
    void samplesChanged();
    void runningChanged();
    void silentChanged();
    void frame();

private:
    enum class ResolveStage { Idle, DefaultSink, Sources };

    void startResolution();
    void handleResolverExit(int exitCode, QProcess::ExitStatus status);
    void startCapture(const QString& target);
    void stopCapture();
    void readPcm();
    void processFrame(const char* pcm);
    void publishFlat();
    void setRunning(bool running);
    void setSilent(bool silent);
    void scheduleRestart();

    static constexpr int sampleRate = 48000;
    static constexpr int windowSamples = 1600;
    static constexpr int outputPoints = 220;
    static constexpr int hopSamples = 800;

    QProcess m_resolver;
    QProcess m_capture;
    QTimer m_restart;
    QTimer m_stale;
    QTimer m_stop;
    QElapsedTimer m_lastFrame;
    QElapsedTimer m_lastNonSilent;
    QByteArray m_pcm;
    qsizetype m_pcmOffset = 0;
    QList<qreal> m_samples;
    QString m_defaultSink;
    QString m_source;
    ResolveStage m_resolveStage = ResolveStage::Idle;
    bool m_explicitResolution = false;
    bool m_active = false;
    bool m_running = false;
    bool m_silent = true;
};
