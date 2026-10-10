#include "previewvideo.h"
#include "previewyuv.h"

#include <QAudioOutput>
#include <QMediaPlayer>
#include <QMutexLocker>
#include <QUrl>
#include <QVideoFrameFormat>
#include <QVideoSink>

#include <algorithm>

namespace {

// The frame as RGBA within cap, converted on the CPU from the formats the
// software decoder emits. Anything else keeps the card's thumbnail: a missing
// preview is nothing, a GPU conversion is the fault this exists to avoid.
QImage imageFromFrame(QVideoFrame frame, QSize cap)
{
    if (!frame.isValid() || !frame.map(QVideoFrame::ReadOnly))
        return QImage();
    const QVideoFrameFormat format = frame.surfaceFormat();
    const QSize size = frame.size();
    QImage out;

    const auto rgb = [&](QImage::Format qtFormat) {
        const QImage wrapped(frame.bits(0), size.width(), size.height(), frame.bytesPerLine(0), qtFormat);
        const bool fits = size.width() <= cap.width() && size.height() <= cap.height();
        out = fits ? wrapped.convertToFormat(QImage::Format_RGBA8888)
                   : wrapped.scaled(cap, Qt::KeepAspectRatio, Qt::SmoothTransformation)
                             .convertToFormat(QImage::Format_RGBA8888);
    };
    const auto yuv = [&](int subX, int subY, bool interleaved, bool vFirst) {
        // vFirst: V before U, NV21 within one plane and YV12 across two.
        PreviewYuvPlanes p;
        p.size = size;
        p.y = frame.bits(0);
        p.yStride = frame.bytesPerLine(0);
        p.subX = subX;
        p.subY = subY;
        if (interleaved) {
            const unsigned char *uv = frame.bits(1);
            p.u = vFirst ? uv + 1 : uv;
            p.v = vFirst ? uv : uv + 1;
            p.uStride = p.vStride = frame.bytesPerLine(1);
            p.uvStep = 2;
        } else {
            p.u = frame.bits(vFirst ? 2 : 1);
            p.v = frame.bits(vFirst ? 1 : 2);
            p.uStride = frame.bytesPerLine(vFirst ? 2 : 1);
            p.vStride = frame.bytesPerLine(vFirst ? 1 : 2);
        }
        p.fullRange = format.colorRange() == QVideoFrameFormat::ColorRange_Full;
        switch (format.colorSpace()) {
        case QVideoFrameFormat::ColorSpace_BT601:
            p.bt709 = false;
            break;
        case QVideoFrameFormat::ColorSpace_BT709:
        case QVideoFrameFormat::ColorSpace_BT2020:
            p.bt709 = true;
            break;
        default:
            p.bt709 = size.height() >= 720;
            break;
        }
        out = previewImageFromYuv(p, cap);
    };

    switch (format.pixelFormat()) {
    case QVideoFrameFormat::Format_YUV420P:
        yuv(1, 1, false, false);
        break;
    case QVideoFrameFormat::Format_YV12:
        yuv(1, 1, false, true);
        break;
    case QVideoFrameFormat::Format_YUV422P:
        yuv(1, 0, false, false);
        break;
    case QVideoFrameFormat::Format_NV12:
        yuv(1, 1, true, false);
        break;
    case QVideoFrameFormat::Format_NV21:
        yuv(1, 1, true, true);
        break;
    case QVideoFrameFormat::Format_RGBA8888:
        rgb(QImage::Format_RGBA8888);
        break;
    case QVideoFrameFormat::Format_RGBX8888:
        rgb(QImage::Format_RGBX8888);
        break;
    case QVideoFrameFormat::Format_BGRA8888:
        rgb(QImage::Format_ARGB32);
        break;
    case QVideoFrameFormat::Format_BGRX8888:
        rgb(QImage::Format_RGB32);
        break;
    default:
        break;
    }
    frame.unmap();
    return out;
}

} // namespace

PreviewVideo::PreviewVideo(QObject *parent)
    : QObject(parent)
{
    m_converter.setMaxThreadCount(1);
}

PreviewVideo::~PreviewVideo()
{
    stop();
    // A conversion still running would post to a dead object.
    m_converter.clear();
    m_converter.waitForDone();
}

void PreviewVideo::ensurePipeline()
{
    if (m_player)
        return;
    m_sink = new QVideoSink(this);
    m_audio = new QAudioOutput(this);
    m_player = new QMediaPlayer(this);
    m_player->setVideoSink(m_sink);
    m_player->setAudioOutput(m_audio);
    m_player->setLoops(QMediaPlayer::Infinite);
}

void PreviewVideo::listen()
{
    QObject::disconnect(m_frameConnection);
    const quint64 generation = ++m_generation;
    m_frameConnection = connect(m_sink, &QVideoSink::videoFrameChanged, this, [this, generation](const QVideoFrame &frame) {
        if (generation == m_generation)
            onFrame(frame);
    });
}

void PreviewVideo::prewarm()
{
    ensurePipeline();
}

void PreviewVideo::play(const QString &path, bool muted, double volume)
{
    stop();
    if (path.isEmpty())
        return;
    ensurePipeline();
    m_audio->setMuted(muted);
    m_audio->setVolume(float(std::clamp(volume, 0.0, 1.0)));
    listen();
    m_active = true;
    const QUrl url = path.contains(QStringLiteral("://")) ? QUrl(path) : QUrl::fromLocalFile(path);
    m_player->setSource(url);
    m_player->play();
}

void PreviewVideo::stop()
{
    m_active = false;
    ++m_generation;
    QObject::disconnect(m_frameConnection);
    if (m_player) {
        m_player->stop();
        m_player->setSource(QUrl());
    }
    m_pending = QVideoFrame();
    m_hasPending = false;
    QMutexLocker lock(&m_mutex);
    m_latest = QImage();
    m_hasFrame = false;
}

void PreviewVideo::onFrame(const QVideoFrame &frame)
{
    if (!frame.isValid())
        return;
    m_pending = frame;
    m_hasPending = true;
    convertPending();
}

void PreviewVideo::convertPending()
{
    if (m_converting || !m_hasPending)
        return;
    m_converting = true;
    m_hasPending = false;
    QVideoFrame frame = std::move(m_pending);
    m_pending = QVideoFrame();
    const quint64 generation = m_generation;
    m_converter.start([this, frame = std::move(frame), generation]() mutable {
        const QImage image = imageFromFrame(std::move(frame), kMaxSize);
        QMetaObject::invokeMethod(this, [this, image, generation]() { onConverted(image, generation); },
                                  Qt::QueuedConnection);
    });
}

void PreviewVideo::onConverted(const QImage &image, quint64 generation)
{
    m_converting = false;
    if (generation == m_generation && !image.isNull()) {
        {
            QMutexLocker lock(&m_mutex);
            m_latest = image;
            m_hasFrame = true;
        }
        Q_EMIT frameReady();
    }
    convertPending();
}

QImage PreviewVideo::takeFrame()
{
    QMutexLocker lock(&m_mutex);
    if (!m_hasFrame)
        return QImage();
    m_hasFrame = false;
    return m_latest;
}
