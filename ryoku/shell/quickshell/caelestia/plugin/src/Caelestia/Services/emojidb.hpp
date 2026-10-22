// SPDX-License-Identifier: GPL-3.0-only
#pragma once

#include <qhash.h>
#include <qobject.h>
#include <qqmlintegration.h>
#include <qset.h>
#include <qstringlist.h>
#include <qvariant.h>
#include <qvector.h>

namespace caelestia::services {

struct EmojiEntry {
    QString ch;
    QString name;
    QString nameLower;
};

class EmojiDb : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(bool loaded READ loaded NOTIFY loadedChanged)
    Q_PROPERTY(int count READ count NOTIFY loadedChanged)

public:
    explicit EmojiDb(QObject* parent = nullptr);

    [[nodiscard]] bool loaded() const;
    [[nodiscard]] int count() const;

    Q_INVOKABLE QVariantList getSortedItems(const QStringList& favourites, int limit = 500) const;
    Q_INVOKABLE QVariantList search(const QString& text, int limit = 500) const;
    Q_INVOKABLE void recordUsage(const QString& ch);
    Q_INVOKABLE int getFrequency(const QString& ch) const;

signals:
    void loadedChanged();

private:
    void loadEmojis();
    bool loadKdeDict(const QString& path, QHash<QString, int>& charMap);
    bool loadTextFile(const QString& path, QHash<QString, int>& charMap);
    void loadFrequencies();
    void saveFrequencies();
    void buildTrigramIndex();

    QString m_freqPath;

    QVector<EmojiEntry> m_emojis;
    QHash<QString, int> m_frequencies;
    // Trigram -> emoji indices
    QHash<QString, QVector<int>> m_trigramIndex;
    bool m_loaded = false;
};

} // namespace caelestia::services
