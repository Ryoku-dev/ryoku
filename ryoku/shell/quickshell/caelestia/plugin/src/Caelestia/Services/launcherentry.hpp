// SPDX-License-Identifier: GPL-3.0-only
#pragma once

#include <QDBusContext>
#include <QHash>
#include <QObject>
#include <QQmlEngine>
#include <QVariantMap>

class QDBusServiceWatcher;

namespace caelestia::services {

class LauncherEntry : public QObject, protected QDBusContext {
    Q_OBJECT
    Q_PROPERTY(int revision READ revision NOTIFY changed)
    QML_ELEMENT
    QML_SINGLETON

public:
    explicit LauncherEntry(QObject* parent = nullptr);

    int revision() const;

    Q_INVOKABLE QVariantMap forApp(const QString& desktopId) const;

signals:
    void changed();

private Q_SLOTS:
    void onUpdate(const QString& appUri, const QVariantMap& properties);
    void onServiceUnregistered(const QString& service);

private:
    struct Source {
        QVariantMap properties;
        quint64 stamp = 0;
    };

    QHash<QString, QHash<QString, Source>> m_sources;
    QDBusServiceWatcher* m_watcher = nullptr;
    quint64 m_stamp = 0;
    int m_revision = 0;
};

} // namespace caelestia::services
