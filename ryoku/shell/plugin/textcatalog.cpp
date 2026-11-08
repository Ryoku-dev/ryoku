#include "textcatalog.hpp"

#include <QDateTime>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonParseError>
#include <QStringList>
#include <utility>

TextCatalog::TextCatalog(QObject *parent)
    : QObject(parent)
{
    connect(&m_watcher, &QFileSystemWatcher::fileChanged, this, [this] {
        ensureWatches();
        reload();
    });
    connect(&m_watcher, &QFileSystemWatcher::directoryChanged, this, [this] {
        const FileStamp stamp = currentStamp();
        ensureWatches();
        if (!(stamp == m_stamp))
            reload();
    });
}

void TextCatalog::setPath(const QString &path)
{
    if (m_path == path)
        return;

    m_path = path;
    resetWatches();
    ensureWatches();
    emit pathChanged();
    reload();
}

QString TextCatalog::value(const QString &key) const
{
    const auto it = m_values.constFind(key);
    return it == m_values.constEnd() ? QString() : it.value();
}

bool TextCatalog::contains(const QString &key) const
{
    return m_values.contains(key);
}

void TextCatalog::reload()
{
    QHash<QString, QString> values;
    bool loaded = false;
    bool failed = false;

    if (m_path.isEmpty()) {
        loaded = true;
    } else {
        QFile file(m_path);
        if (!file.open(QIODevice::ReadOnly)) {
            failed = true;
        } else {
            QJsonParseError error;
            const QJsonDocument document = QJsonDocument::fromJson(file.readAll(), &error);
            if (error.error != QJsonParseError::NoError || !document.isObject()) {
                failed = true;
            } else {
                const QJsonObject object = document.object();
                values.reserve(object.size());
                for (auto it = object.constBegin(); it != object.constEnd(); ++it) {
                    if (it.value().isString())
                        values.insert(it.key(), it.value().toString());
                }
                loaded = true;
            }
        }
    }

    ensureWatches();
    m_stamp = currentStamp();
    apply(std::move(values), loaded, failed);
}

void TextCatalog::resetWatches()
{
    const QStringList files = m_watcher.files();
    if (!files.isEmpty())
        m_watcher.removePaths(files);
    const QStringList directories = m_watcher.directories();
    if (!directories.isEmpty())
        m_watcher.removePaths(directories);
}

void TextCatalog::ensureWatches()
{
    if (m_path.isEmpty())
        return;

    const QFileInfo info(m_path);
    QString directory = info.absolutePath();
    while (!QFileInfo::exists(directory)) {
        const QString parent = QFileInfo(directory).absolutePath();
        if (parent == directory)
            break;
        directory = parent;
    }

    const QStringList watchedDirectories = m_watcher.directories();
    for (const QString &watched : watchedDirectories) {
        if (watched != directory)
            m_watcher.removePath(watched);
    }
    if (!m_watcher.directories().contains(directory) && QFileInfo::exists(directory))
        m_watcher.addPath(directory);
    if (!m_watcher.files().contains(m_path) && info.exists())
        m_watcher.addPath(m_path);
}

TextCatalog::FileStamp TextCatalog::currentStamp() const
{
    if (m_path.isEmpty())
        return {};

    const QFileInfo info(m_path);
    if (!info.exists())
        return {};
    return {true, info.size(), info.lastModified().toMSecsSinceEpoch()};
}

void TextCatalog::apply(QHash<QString, QString> values, bool loaded, bool failed)
{
    const bool valuesChanged = m_values != values;
    const bool loadedChanged = m_loaded != loaded;
    const bool failedChanged = m_failed != failed;
    if (!valuesChanged && !loadedChanged && !failedChanged)
        return;

    const int oldCount = count();
    m_values = std::move(values);
    m_loaded = loaded;
    m_failed = failed;
    ++m_revision;

    if (oldCount != count())
        emit countChanged();
    if (loadedChanged)
        emit this->loadedChanged();
    if (failedChanged)
        emit this->failedChanged();
    emit revisionChanged();
}
