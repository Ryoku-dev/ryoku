// SPDX-License-Identifier: GPL-3.0-only
#pragma once

#include <qobject.h>
#include <qprocess.h>
#include <qqmlintegration.h>
#include <qset.h>
#include <qstring.h>
#include <qvariant.h>

namespace caelestia::services {

class ClipboardManager : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(QVariantList items READ items NOTIFY itemsChanged)
    Q_PROPERTY(QString imageCacheDir READ imageCacheDir CONSTANT)
    Q_PROPERTY(bool available READ available NOTIFY availableChanged)
    Q_PROPERTY(QVariantList pinnedItems READ pinnedItems NOTIFY pinnedItemsChanged)

public:
    explicit ClipboardManager(QObject* parent = nullptr);

    [[nodiscard]] QVariantList items() const;
    [[nodiscard]] QString imageCacheDir() const;
    [[nodiscard]] bool available() const;
    [[nodiscard]] QVariantList pinnedItems() const;

    Q_INVOKABLE void reload();
    [[nodiscard]] Q_INVOKABLE bool isImageCached(int id) const;
    Q_INVOKABLE void decodeImage(int id, const QString& outPath);
    Q_INVOKABLE void clearHistory();

    Q_INVOKABLE void pin(int id);
    Q_INVOKABLE void unpin(int pinId);
    /// Pinned entries are no longer in cliphist, so `cliphist decode` cannot
    /// bring them back — the stored bytes go to wl-copy directly.
    Q_INVOKABLE void copyPinned(int pinId);

signals:
    void itemsChanged();
    /// Emitted after the image file for `id` has been fully written to `path`.
    void imageReady(int id, const QString& path);
    void clearHistoryFinished(bool success);
    void availableChanged();
    void pinnedItemsChanged();
    void pinFailed(int id);

private:
    void setAvailable(bool available);

    void loadPins();
    void savePins();
    QString pinFilePath(int pinId, bool isImage) const;

    QVariantList m_items;
    bool m_available = true;
    QProcess* m_listProc = nullptr;
    QProcess* m_wipeProc = nullptr;
    QString m_imageCacheDir;

    QVariantList m_pinnedItems;
    QString m_pinDir;
    int m_nextPinId = 1;
    QSet<int> m_activeDecodes;
};

} // namespace caelestia::services
