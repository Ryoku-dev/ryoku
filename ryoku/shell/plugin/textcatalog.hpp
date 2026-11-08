#pragma once

#include <QFileSystemWatcher>
#include <QHash>
#include <QObject>
#include <QString>
#include <QtQml/qqmlregistration.h>

class TextCatalog : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(QString path READ path WRITE setPath NOTIFY pathChanged)
    Q_PROPERTY(int count READ count NOTIFY countChanged)
    Q_PROPERTY(int revision READ revision NOTIFY revisionChanged)
    Q_PROPERTY(bool loaded READ loaded NOTIFY loadedChanged)
    Q_PROPERTY(bool failed READ failed NOTIFY failedChanged)

public:
    explicit TextCatalog(QObject *parent = nullptr);

    QString path() const { return m_path; }
    void setPath(const QString &path);
    int count() const { return int(m_values.size()); }
    int revision() const { return m_revision; }
    bool loaded() const { return m_loaded; }
    bool failed() const { return m_failed; }

    Q_INVOKABLE QString value(const QString &key) const;
    Q_INVOKABLE bool contains(const QString &key) const;

signals:
    void pathChanged();
    void countChanged();
    void revisionChanged();
    void loadedChanged();
    void failedChanged();

private:
    struct FileStamp {
        bool exists = false;
        qint64 size = -1;
        qint64 modifiedMs = 0;

        bool operator==(const FileStamp &) const = default;
    };

    void reload();
    void resetWatches();
    void ensureWatches();
    FileStamp currentStamp() const;
    void apply(QHash<QString, QString> values, bool loaded, bool failed);

    QFileSystemWatcher m_watcher;
    QHash<QString, QString> m_values;
    QString m_path;
    FileStamp m_stamp;
    int m_revision = 0;
    bool m_loaded = false;
    bool m_failed = false;
};
