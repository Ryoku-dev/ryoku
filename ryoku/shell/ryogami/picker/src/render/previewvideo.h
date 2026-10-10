#pragma once

#include <QImage>
#include <QMutex>
#include <QObject>
#include <QString>
#include <QThreadPool>
#include <QVideoFrame>

class QAudioOutput;
class QMediaPlayer;
class QVideoSink;

// Plays a clip and hands its frames over as RGBA images, converted on the CPU
// by a worker of its own. QVideoFrame::toImage() is never used: it converts on
// the GPU through a thread-local QRhi of Qt Multimedia's own, and called from
// the render path that leaves a second OpenGL context current under the scene
// graph, which on the NVIDIA driver tore the whole picker into shards.
class PreviewVideo : public QObject
{
    Q_OBJECT
public:
    explicit PreviewVideo(QObject *parent = nullptr);
    ~PreviewVideo() override;

    // The largest frame delivered; the render node's preview texture is this size for its whole life.
    static constexpr QSize kMaxSize{640, 360};

    void play(const QString &path, bool muted, double volume);
    void stop();
    // Creating the first player blocks for most of a second, so it happens while the picker is hidden.
    void prewarm();

    // Null when no new frame arrived since the last take.
    QImage takeFrame();
    bool active() const { return m_active; }

Q_SIGNALS:
    void frameReady();

private:
    void ensurePipeline();
    // Frames arrive queued from the decoder thread; a new generation drops the last clip's.
    void listen();
    void onFrame(const QVideoFrame &frame);
    // One conversion in flight; a frame arriving meanwhile replaces the one waiting.
    void convertPending();
    void onConverted(const QImage &image, quint64 generation);

    QMediaPlayer *m_player = nullptr;
    QVideoSink *m_sink = nullptr;
    QAudioOutput *m_audio = nullptr;
    QThreadPool m_converter;

    mutable QMutex m_mutex;
    QImage m_latest;
    bool m_hasFrame = false;
    bool m_active = false;
    quint64 m_generation = 0;
    QVideoFrame m_pending;
    bool m_hasPending = false;
    bool m_converting = false;
    QMetaObject::Connection m_frameConnection;
};
