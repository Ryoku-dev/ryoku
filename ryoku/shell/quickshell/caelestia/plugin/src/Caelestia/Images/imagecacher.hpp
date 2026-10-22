#pragma once

#include <qmutex.h>
#include <qobject.h>
#include <qset.h>
#include <qsize.h>
#include <qstring.h>

namespace caelestia::images {

class ImageCacher : public QObject {
    Q_OBJECT

public:
    enum class FillMode {
        Crop,
        Fit,
        Stretch,
    };

    static ImageCacher* instance();

    static const QString& cacheDir();
    static QString cachePathFor(const QString& sourcePath, const QSize& size, FillMode fillMode);

    void schedule(const QString& sourcePath, const QSize& size, FillMode fillMode);
    void schedule(const QString& sourcePath, const QString& cachePath, const QSize& size, FillMode fillMode);

    // The blocking form of the same job: builds the cache file if it is missing and
    // returns when it is there. An image provider request runs on its own thread and
    // cannot wait for a pooled task, so it calls this rather than scheduling one.
    static void runJob(const QString& sourcePath, const QString& cachePath, const QSize& size, FillMode fillMode);

private:
    explicit ImageCacher(QObject* parent = nullptr);

    QMutex m_mutex;
    QSet<QString> m_inflight;
};

} // namespace caelestia::images
