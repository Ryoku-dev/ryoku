#include "cutils.hpp"

#include <QtConcurrent/qtconcurrentrun.h>
#include <QtQuick/qquickitemgrabresult.h>
#include <QtQuick/qquickwindow.h>
#include <qcryptographichash.h>
#include <qdir.h>
#include <qfile.h>
#include <qfileinfo.h>
#include <qfuturewatcher.h>
#include <qloggingcategory.h>
#include <qqmlengine.h>
#include <qregularexpression.h>
#include <qsavefile.h>
#include <qtextstream.h>

#include <QCursor>
#include <QGuiApplication>
#include <QStandardPaths>

#include "util/metaenum.hpp"

Q_LOGGING_CATEGORY(lcCUtils, "caelestia.cutils", QtInfoMsg)

namespace caelestia {

class CUtils::Private {
public:
    // shortcut: only the modifiers of the focused window are known, never lock state or
    // another app's keys, upgrade by reading evdev when a surface needs them unfocused.
    static bool pressed(Qt::Key key) {
        const auto mods = QGuiApplication::queryKeyboardModifiers();
        switch (key) {
        case Qt::Key_Alt:
        case Qt::Key_AltGr:
            return mods.testFlag(Qt::AltModifier);
        case Qt::Key_Meta:
        case Qt::Key_Super_L:
        case Qt::Key_Super_R:
            return mods.testFlag(Qt::MetaModifier);
        case Qt::Key_Control:
            return mods.testFlag(Qt::ControlModifier);
        case Qt::Key_Shift:
            return mods.testFlag(Qt::ShiftModifier);
        default:
            return false;
        }
    }
};

CUtils::CUtils(QObject* parent)
    : QObject(parent)
    , d(new Private) {}

void CUtils::saveItem(QQuickItem* target, const QUrl& path) {
    this->saveItem(target, path, QRect(), QJSValue(), QJSValue());
}

void CUtils::saveItem(QQuickItem* target, const QUrl& path, const QRect& rect) {
    this->saveItem(target, path, rect, QJSValue(), QJSValue());
}

void CUtils::saveItem(QQuickItem* target, const QUrl& path, QJSValue onSaved) {
    this->saveItem(target, path, QRect(), onSaved, QJSValue());
}

void CUtils::saveItem(QQuickItem* target, const QUrl& path, QJSValue onSaved, QJSValue onFailed) {
    this->saveItem(target, path, QRect(), onSaved, onFailed);
}

void CUtils::saveItem(QQuickItem* target, const QUrl& path, const QRect& rect, QJSValue onSaved) {
    this->saveItem(target, path, rect, onSaved, QJSValue());
}

void CUtils::saveItem(QQuickItem* target, const QUrl& path, const QRect& rect, QJSValue onSaved, QJSValue onFailed) {
    if (!target) {
        qCWarning(lcCUtils) << "saveItem: a target is required";
        return;
    }

    if (!path.isLocalFile()) {
        qCWarning(lcCUtils) << "saveItem:" << path << "is not a local file";
        return;
    }

    if (!target->window()) {
        qCWarning(lcCUtils) << "saveItem: unable to save target" << target << "without a window";
        return;
    }

    auto scaledRect = rect;
    const qreal scale = target->window()->devicePixelRatio();
    if (rect.isValid() && !qFuzzyCompare(scale + 1.0, 2.0)) {
        scaledRect =
            QRectF(rect.left() * scale, rect.top() * scale, rect.width() * scale, rect.height() * scale).toRect();
    }

    const QSharedPointer<const QQuickItemGrabResult> grabResult = target->grabToImage();
    if (!grabResult) {
        // grabToImage() returns null when the item's window is hidden or otherwise
        // not renderable; connecting to a null result would warn and never fire.
        qCWarning(lcCUtils) << "saveItem: failed to grab" << target;
        return;
    }

    QObject::connect(grabResult.data(), &QQuickItemGrabResult::ready, this,
        [grabResult, scaledRect, path, onSaved, onFailed, this]() {
            const auto future = QtConcurrent::run([=]() {
                QImage image = grabResult->image();

                if (scaledRect.isValid()) {
                    image = image.copy(scaledRect);
                }

                const QString file = path.toLocalFile();
                const QString parent = QFileInfo(file).absolutePath();
                return QDir().mkpath(parent) && image.save(file);
            });

            auto* watcher = new QFutureWatcher<bool>(this);
            auto* engine = qmlEngine(this);

            QObject::connect(watcher, &QFutureWatcher<bool>::finished, this, [=]() {
                if (watcher->result()) {
                    if (onSaved.isCallable()) {
                        QJSValueList args = { QJSValue(path.toLocalFile()) };
                        if (engine) {
                            args << engine->toScriptValue(QVariant::fromValue(path));
                        }
                        onSaved.call(args);
                    }
                } else {
                    qCWarning(lcCUtils) << "saveItem: failed to save" << path;
                    if (onFailed.isCallable()) {
                        if (engine) {
                            onFailed.call({ engine->toScriptValue(QVariant::fromValue(path)) });
                        } else {
                            onFailed.call();
                        }
                    }
                }
                watcher->deleteLater();
            });
            watcher->setFuture(future);
        });
}

bool CUtils::copyFile(const QUrl& source, const QUrl& target, bool overwrite) {
    if (!source.isLocalFile()) {
        qCWarning(lcCUtils) << "copyFile: source" << source << "is not a local file";
        return false;
    }
    if (!target.isLocalFile()) {
        qCWarning(lcCUtils) << "copyFile: target" << target << "is not a local file";
        return false;
    }

    if (overwrite && QFile::exists(target.toLocalFile())) {
        if (!QFile::remove(target.toLocalFile())) {
            qCWarning(lcCUtils) << "copyFile: overwrite was specified but failed to remove" << target.toLocalFile();
            return false;
        }
    }

    return QFile::copy(source.toLocalFile(), target.toLocalFile());
}

bool CUtils::deleteFile(const QUrl& path) {
    if (!path.isLocalFile()) {
        qCWarning(lcCUtils) << "deleteFile: path" << path << "is not a local file";
        return false;
    }

    return QFile::remove(path.toLocalFile());
}

QString CUtils::toLocalFile(const QUrl& url) {
    if (!url.isLocalFile()) {
        qCWarning(lcCUtils) << "toLocalFile: given url is not a local file" << url;
        return QString();
    }

    return url.toLocalFile();
}

QString CUtils::sha256(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly)) {
        qCWarning(lcCUtils) << "sha256: failed to open" << path;
        return QString();
    }

    QCryptographicHash hash(QCryptographicHash::Sha256);
    hash.addData(&file);
    file.close();

    return QString::fromLatin1(hash.result().toHex());
}

bool CUtils::setDesktopEntryKey(const QString& path, const QString& key, const QString& value) {
    if (path.isEmpty() || key.isEmpty()) {
        return false;
    }

    QFile file(path);
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text)) {
        return false;
    }

    QTextStream in(&file);
    QStringList lines;
    int headerIndex = -1;
    QString currentGroup;
    bool found = false;

    while (!in.atEnd()) {
        QString line = in.readLine();
        const QString trimmed = line.trimmed();
        if (trimmed.startsWith(QLatin1Char('['))) {
            currentGroup = trimmed;
            if (currentGroup == QLatin1String("[Desktop Entry]") && headerIndex == -1) {
                headerIndex = lines.size();
            }
        } else if (currentGroup == QLatin1String("[Desktop Entry]")) {
            const int eq = trimmed.indexOf(QLatin1Char('='));
            if (eq > 0 && trimmed.left(eq).trimmed() == key) {
                line = key + QLatin1Char('=') + value;
                found = true;
            }
        }
        lines.append(line);
    }
    file.close();

    if (!found) {
        if (headerIndex == -1) {
            lines.append(QStringLiteral("[Desktop Entry]"));
            lines.append(key + QLatin1Char('=') + value);
        } else {
            lines.insert(headerIndex + 1, key + QLatin1Char('=') + value);
        }
    }

    QSaveFile outFile(path);
    if (!outFile.open(QIODevice::WriteOnly | QIODevice::Text)) {
        return false;
    }

    QTextStream out(&outFile);
    for (int i = 0; i < lines.size(); ++i) {
        out << lines.at(i) << "\n";
    }

    return outFile.commit();
}

void CUtils::enableBlurBehind(QQuickWindow* window, bool enable) {
    // Blur comes from the compositor's layer rules, not from a per-window request.
    Q_UNUSED(window);
    Q_UNUSED(enable);
}

qreal CUtils::clamp(qreal value, qreal min, qreal max) {
    return qBound(min, value, max);
}

void CUtils::setCursorPos(int x, int y) {
    QCursor::setPos(x, y);
}

QString CUtils::enumToString(const QVariant& value) {
    const auto type = value.metaType();
    if (!util::isSupportedEnum(type)) {
        qCWarning(lcCUtils, "enumToString: %s is not a supported enum", type.isValid() ? type.name() : "the value");
        return {};
    }

    const auto* key = util::enumKeyFor(util::metaEnumFor(type), value);
    if (!key) {
        qCWarning(lcCUtils, "enumToString: no enumerator of %s has the value %lld", type.name(), value.toLongLong());
        return {};
    }

    return QString::fromUtf8(key);
}

#ifndef CAELESTIA_VERSION
#define CAELESTIA_VERSION ""
#endif

QString CUtils::version() const {
    return QStringLiteral(CAELESTIA_VERSION);
}

QString CUtils::qtVersion() const {
    return QStringLiteral(QT_VERSION_STR);
}

bool CUtils::capsLock() const {
    return false;
}

bool CUtils::numLock() const {
    return false;
}

bool CUtils::isKeyPressed(int key) const {
    return Private::pressed(static_cast<Qt::Key>(key));
}

bool CUtils::isAltPressed() const {
    return Private::pressed(Qt::Key_Alt) || Private::pressed(Qt::Key_AltGr);
}

bool CUtils::isMetaPressed() const {
    return Private::pressed(Qt::Key_Meta) || Private::pressed(Qt::Key_Super_L) ||
           Private::pressed(Qt::Key_Super_R);
}

bool CUtils::isCtrlPressed() const {
    return Private::pressed(Qt::Key_Control);
}

bool CUtils::isShiftPressed() const {
    return Private::pressed(Qt::Key_Shift);
}

bool CUtils::isShortcutModifierPressed(const QString& shortcutKey) const {
    if (shortcutKey.isEmpty()) {
        return isAltPressed();
    }
    const QString upper = shortcutKey.toUpper();
    const bool hasAlt = upper.contains(QLatin1String("ALT"));
    const bool hasMeta = upper.contains(QLatin1String("META")) || upper.contains(QLatin1String("SUPER")) ||
                         upper.contains(QLatin1String("WIN"));
    const bool hasCtrl = upper.contains(QLatin1String("CTRL")) || upper.contains(QLatin1String("CONTROL"));

    if (hasAlt && isAltPressed())
        return true;
    if (hasMeta && isMetaPressed())
        return true;
    if (hasCtrl && isCtrlPressed())
        return true;

    if (!hasAlt && !hasMeta && !hasCtrl) {
        if (upper.contains(QLatin1String("SHIFT"))) {
            return isShiftPressed();
        }
        return isAltPressed();
    }

    return false;
}

namespace {

template <typename Predicate> QQuickItem* findChildDfs(QQuickItem* root, Predicate&& match) {
    const auto children = root->childItems();
    for (QQuickItem* const child : children) {
        if (match(child)) {
            return child;
        }
        if (QQuickItem* const found = findChildDfs(child, match)) {
            return found;
        }
    }
    return nullptr;
}

template <typename Predicate> void findChildrenDfs(QQuickItem* root, Predicate&& match, QList<QQuickItem*>& out) {
    const auto children = root->childItems();
    for (QQuickItem* const child : children) {
        if (match(child)) {
            out.append(child);
        }
        findChildrenDfs(child, match, out);
    }
}

} // namespace

QQuickItem* CUtils::findChild(QQuickItem* root, const QString& name) {
    if (!root) {
        return nullptr;
    }

    return findChildDfs(root, [&name](const QQuickItem* item) {
        return item->objectName() == name;
    });
}

QList<QQuickItem*> CUtils::findChildren(QQuickItem* root, const QString& name) {
    QList<QQuickItem*> children;
    if (root) {
        findChildrenDfs(
            root,
            [&name](const QQuickItem* item) {
                return item->objectName() == name;
            },
            children);
    }
    return children;
}

QList<QQuickItem*> CUtils::findChildrenMatching(QQuickItem* root, const QString& pattern) {
    QList<QQuickItem*> children;
    if (root) {
        const QRegularExpression re(pattern);
        findChildrenDfs(
            root,
            [&re](const QQuickItem* item) {
                return re.match(item->objectName()).hasMatch();
            },
            children);
    }
    return children;
}

} // namespace caelestia
