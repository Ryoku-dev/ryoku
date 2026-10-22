#pragma once

#include <QtQuick/qquickitem.h>
#include <QtQuick/qquickwindow.h>
#include <qobject.h>
#include <qqmlintegration.h>

namespace caelestia {

class CUtils : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(QString version READ version CONSTANT)
    Q_PROPERTY(QString qtVersion READ qtVersion CONSTANT)
    Q_PROPERTY(bool capsLock READ capsLock NOTIFY capsLockChanged)
    Q_PROPERTY(bool numLock READ numLock NOTIFY numLockChanged)
    Q_PROPERTY(bool altPressed READ isAltPressed NOTIFY altPressedChanged)
    Q_PROPERTY(bool metaPressed READ isMetaPressed NOTIFY metaPressedChanged)
    Q_PROPERTY(bool ctrlPressed READ isCtrlPressed NOTIFY ctrlPressedChanged)
    Q_PROPERTY(bool shiftPressed READ isShiftPressed NOTIFY shiftPressedChanged)

public:
    explicit CUtils(QObject* parent = nullptr);
    // clang-format off
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path);
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path, const QRect& rect);
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path, QJSValue onSaved);
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path, QJSValue onSaved, QJSValue onFailed);
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path, const QRect& rect, QJSValue onSaved);
    Q_INVOKABLE void saveItem(QQuickItem* target, const QUrl& path, const QRect& rect, QJSValue onSaved, QJSValue onFailed);
    // clang-format on

    Q_INVOKABLE static bool copyFile(const QUrl& source, const QUrl& target, bool overwrite = true);
    Q_INVOKABLE static bool deleteFile(const QUrl& path);
    Q_INVOKABLE static QString toLocalFile(const QUrl& url);
    Q_INVOKABLE static QString sha256(const QString& path);
    Q_INVOKABLE static bool setDesktopEntryKey(const QString& path, const QString& key, const QString& value);

    Q_INVOKABLE static void enableBlurBehind(QQuickWindow* window, bool enable = true);

    Q_INVOKABLE static qreal clamp(qreal value, qreal min, qreal max);
    Q_INVOKABLE static void setCursorPos(int x, int y);

    Q_INVOKABLE static QString enumToString(const QVariant& value);

    Q_INVOKABLE bool isKeyPressed(int key) const;
    Q_INVOKABLE bool isAltPressed() const;
    Q_INVOKABLE bool isMetaPressed() const;
    Q_INVOKABLE bool isCtrlPressed() const;
    Q_INVOKABLE bool isShiftPressed() const;
    Q_INVOKABLE bool isShortcutModifierPressed(const QString& shortcutKey) const;

    Q_INVOKABLE static QQuickItem* findChild(QQuickItem* root, const QString& name);
    Q_INVOKABLE static QList<QQuickItem*> findChildren(QQuickItem* root, const QString& name);
    Q_INVOKABLE static QList<QQuickItem*> findChildrenMatching(QQuickItem* root, const QString& pattern);

    [[nodiscard]] QString version() const;
    [[nodiscard]] QString qtVersion() const;
    [[nodiscard]] bool capsLock() const;
    [[nodiscard]] bool numLock() const;

signals:
    void capsLockChanged();
    void numLockChanged();
    void altPressedChanged(bool pressed);
    void metaPressedChanged(bool pressed);
    void ctrlPressedChanged(bool pressed);
    void shiftPressedChanged(bool pressed);
    void keyPressed(int key, bool pressed);
    void modifierReleased();

private:
    class Private;
    Private* d;
};

} // namespace caelestia
