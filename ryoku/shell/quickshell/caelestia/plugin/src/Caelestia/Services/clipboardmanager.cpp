// SPDX-License-Identifier: GPL-3.0-only
#include "clipboardmanager.hpp"

#include <qdir.h>
#include <qfile.h>
#include <qfileinfo.h>
#include <qjsonarray.h>
#include <qjsondocument.h>
#include <qjsonobject.h>
#include <qloggingcategory.h>
#include <qregularexpression.h>
#include <qsavefile.h>

#include <QStandardPaths>

#include "../Config/launcherconfig.hpp"
#include "../Config/rootnodes.hpp"

Q_LOGGING_CATEGORY(lcClipboard, "caelestia.services.clipboard", QtInfoMsg)

namespace caelestia::services {

ClipboardManager::ClipboardManager(QObject* parent)
    : QObject(parent) {
    QString runtimeDir = QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation);
    if (runtimeDir.isEmpty()) {
        runtimeDir = QStandardPaths::writableLocation(QStandardPaths::TempLocation) + QStringLiteral("/caelestia-") +
                     qEnvironmentVariable("USER");
    }
    m_imageCacheDir = runtimeDir + QStringLiteral("/clipboard");

    // Pins live in the state dir, not the runtime dir: they have to outlive a
    // reboot, and they must not sit inside the cache clearHistory() wipes.
    const auto stateDir = qEnvironmentVariable("XDG_STATE_HOME", QDir::homePath() + QStringLiteral("/.local/state"));
    m_pinDir = stateDir + QStringLiteral("/caelestia/clipboard-pins");
    loadPins();
}

QVariantList ClipboardManager::items() const {
    return m_items;
}

QString ClipboardManager::imageCacheDir() const {
    return m_imageCacheDir;
}

bool ClipboardManager::available() const {
    return m_available;
}

void ClipboardManager::setAvailable(bool available) {
    if (m_available == available) {
        return;
    }
    m_available = available;
    emit availableChanged();
}

QVariantList ClipboardManager::pinnedItems() const {
    return m_pinnedItems;
}

QString ClipboardManager::pinFilePath(int pinId, bool isImage) const {
    return m_pinDir + QStringLiteral("/") + QString::number(pinId) +
           (isImage ? QStringLiteral(".png") : QStringLiteral(".bin"));
}

void ClipboardManager::loadPins() {
    QFile index(m_pinDir + QStringLiteral("/index.json"));
    if (!index.exists() || !index.open(QIODevice::ReadOnly)) {
        return;
    }

    const auto doc = QJsonDocument::fromJson(index.readAll());
    index.close();
    if (!doc.isObject()) {
        qCWarning(lcClipboard) << "Ignoring malformed clipboard pin index";
        return;
    }

    const auto obj = doc.object();
    m_nextPinId = obj.value(QStringLiteral("nextPinId")).toInt(1);

    QVariantList loaded;
    const auto entries = obj.value(QStringLiteral("pins")).toArray();
    for (const auto& value : entries) {
        const auto entry = value.toObject();
        const int pinId = entry.value(QStringLiteral("pinId")).toInt(-1);
        if (pinId < 0) {
            continue;
        }
        const bool isImage = entry.value(QStringLiteral("isImage")).toBool();

        // Drop entries whose payload went missing rather than showing a pin
        // that cannot be pasted.
        const auto path = pinFilePath(pinId, isImage);
        if (!QFileInfo::exists(path)) {
            qCWarning(lcClipboard) << "Dropping clipboard pin with missing payload:" << path;
            continue;
        }

        loaded.append(QVariantMap{
            { QStringLiteral("pinId"), pinId },
            { QStringLiteral("preview"), entry.value(QStringLiteral("preview")).toString() },
            { QStringLiteral("isImage"), isImage },
            { QStringLiteral("imagePath"), isImage ? path : QString() },
        });
    }

    m_pinnedItems = loaded;
    if (!m_pinnedItems.isEmpty()) {
        emit pinnedItemsChanged();
    }
}

void ClipboardManager::savePins() {
    if (!QDir().mkpath(m_pinDir)) {
        qCWarning(lcClipboard) << "Failed to create clipboard pin directory:" << m_pinDir;
        return;
    }

    QJsonArray entries;
    for (const auto& value : std::as_const(m_pinnedItems)) {
        const auto map = value.toMap();
        entries.append(QJsonObject{
            { QStringLiteral("pinId"), map.value(QStringLiteral("pinId")).toInt() },
            { QStringLiteral("preview"), map.value(QStringLiteral("preview")).toString() },
            { QStringLiteral("isImage"), map.value(QStringLiteral("isImage")).toBool() },
        });
    }

    const QJsonObject root{
        { QStringLiteral("nextPinId"), m_nextPinId },
        { QStringLiteral("pins"), entries },
    };

    QFile index(m_pinDir + QStringLiteral("/index.json"));
    if (!index.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        qCWarning(lcClipboard) << "Failed to write clipboard pin index:" << index.fileName();
        return;
    }
    index.write(QJsonDocument(root).toJson(QJsonDocument::Compact));
    index.close();
}

void ClipboardManager::pin(int id) {
    QVariantMap source;
    for (const auto& value : std::as_const(m_items)) {
        const auto map = value.toMap();
        if (map.value(QStringLiteral("id")).toInt() == id) {
            source = map;
            break;
        }
    }
    if (source.isEmpty()) {
        qCWarning(lcClipboard) << "Refusing to pin unknown clipboard entry" << id;
        emit pinFailed(id);
        return;
    }

    if (!QDir().mkpath(m_pinDir)) {
        qCWarning(lcClipboard) << "Failed to create clipboard pin directory:" << m_pinDir;
        emit pinFailed(id);
        return;
    }

    const bool isImage = source.value(QStringLiteral("isImage")).toBool();
    const int pinId = m_nextPinId;
    const auto path = pinFilePath(pinId, isImage);
    const auto preview = source.value(QStringLiteral("preview")).toString();

    auto* proc = new QProcess(this);
    proc->setProgram(QStringLiteral("cliphist"));
    proc->setArguments({ QStringLiteral("decode"), QString::number(id) });

    connect(proc, &QProcess::finished, this,
        [this, proc, id, pinId, path, preview, isImage](int exitCode, QProcess::ExitStatus status) {
            const auto data = proc->readAllStandardOutput();
            proc->deleteLater();

            if (status == QProcess::CrashExit || exitCode != 0 || data.isEmpty()) {
                qCWarning(lcClipboard) << "cliphist decode failed while pinning entry" << id;
                emit pinFailed(id);
                return;
            }

            QFile f(path);
            if (!f.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
                qCWarning(lcClipboard) << "Failed to write clipboard pin payload:" << path;
                emit pinFailed(id);
                return;
            }
            f.write(data);
            f.close();

            m_nextPinId = pinId + 1;
            m_pinnedItems.append(QVariantMap{
                { QStringLiteral("pinId"), pinId },
                { QStringLiteral("preview"), preview },
                { QStringLiteral("isImage"), isImage },
                { QStringLiteral("imagePath"), isImage ? path : QString() },
            });
            savePins();
            emit pinnedItemsChanged();
        });

    connect(proc, &QProcess::errorOccurred, this, [this, proc, id](QProcess::ProcessError err) {
        qCWarning(lcClipboard) << "cliphist decode process error while pinning" << id << ":" << err;
        if (err == QProcess::FailedToStart) {
            setAvailable(false);
            proc->deleteLater();
            emit pinFailed(id);
        }
    });

    proc->start();
}

void ClipboardManager::unpin(int pinId) {
    for (int i = 0; i < m_pinnedItems.size(); ++i) {
        const auto map = m_pinnedItems.at(i).toMap();
        if (map.value(QStringLiteral("pinId")).toInt() != pinId) {
            continue;
        }

        const auto path = pinFilePath(pinId, map.value(QStringLiteral("isImage")).toBool());
        if (QFileInfo::exists(path) && !QFile::remove(path)) {
            qCWarning(lcClipboard) << "Failed to remove clipboard pin payload:" << path;
        }

        m_pinnedItems.removeAt(i);
        savePins();
        emit pinnedItemsChanged();
        return;
    }
}

void ClipboardManager::copyPinned(int pinId) {
    for (const auto& value : std::as_const(m_pinnedItems)) {
        const auto map = value.toMap();
        if (map.value(QStringLiteral("pinId")).toInt() != pinId) {
            continue;
        }

        const bool isImage = map.value(QStringLiteral("isImage")).toBool();
        QFile f(pinFilePath(pinId, isImage));
        if (!f.open(QIODevice::ReadOnly)) {
            qCWarning(lcClipboard) << "Failed to read clipboard pin payload:" << f.fileName();
            return;
        }
        const auto data = f.readAll();
        f.close();

        auto* proc = new QProcess(this);
        proc->setProgram(QStringLiteral("wl-copy"));
        proc->setArguments(
            isImage ? QStringList{ QStringLiteral("--type"), QStringLiteral("image/png") } : QStringList{});

        connect(proc, &QProcess::finished, proc, &QProcess::deleteLater);
        connect(proc, &QProcess::errorOccurred, this, [proc](QProcess::ProcessError err) {
            qCWarning(lcClipboard) << "wl-copy process error:" << err;
            if (err == QProcess::FailedToStart) {
                proc->deleteLater();
            }
        });

        proc->start();
        proc->write(data);
        proc->closeWriteChannel();
        return;
    }

    qCWarning(lcClipboard) << "Refusing to copy unknown clipboard pin" << pinId;
}

bool ClipboardManager::isImageCached(int id) const {
    const QString path = m_imageCacheDir + QStringLiteral("/") + QString::number(id) + QStringLiteral(".png");
    const QFileInfo fi(path);
    return fi.exists() && fi.size() > 0;
}

void ClipboardManager::reload() {
    if (m_listProc && m_listProc->state() != QProcess::NotRunning) {
        m_listProc->kill();
        m_listProc->waitForFinished(200);
    }

    auto* proc = new QProcess(this);
    m_listProc = proc;
    proc->setProgram(QStringLiteral("cliphist"));
    proc->setArguments({ QStringLiteral("list") });

    const auto release = [this, proc] {
        if (m_listProc == proc) {
            m_listProc = nullptr;
        }
        proc->deleteLater();
    };

    connect(proc, &QProcess::finished, this, [this, proc, release](int exitCode, QProcess::ExitStatus status) {
        const bool current = m_listProc == proc;
        const auto output = proc->readAllStandardOutput();
        release();

        if (!current) {
            return;
        }

        if (status == QProcess::CrashExit || exitCode != 0) {
            qCWarning(lcClipboard) << "cliphist list failed with exit code" << exitCode;
            m_items.clear();
            emit itemsChanged();
            return;
        }

        static const QRegularExpression imageRe(
            QStringLiteral(
                R"(\[\[ binary data [\d\.]+\s*(?:B|KiB|MiB|GiB)\s+(?:png|jpe?g|webp|gif|bmp|ico|tiff|svg)(?:\s+\d+x\d+)?\s*\]\])"),
            QRegularExpression::CaseInsensitiveOption);

        QVariantList result;
        const auto lines = output.split('\n');
        result.reserve(lines.size());

        const int maxEntries = caelestia::config::ConfigSingleton::instance()->launcher()->clipboardMaxEntries();
        int count = 0;

        for (const auto& rawLine : lines) {
            if (count >= maxEntries)
                break;

            const auto line = QString::fromUtf8(rawLine);
            if (line.isEmpty())
                continue;

            const auto tabIdx = line.indexOf(u'\t');
            if (tabIdx < 0)
                continue;

            bool ok = false;
            const int id = line.left(tabIdx).toInt(&ok);
            if (!ok)
                continue;

            const auto preview = line.mid(tabIdx + 1);
            const bool isImage = imageRe.match(preview).hasMatch();

            result.append(QVariantMap{
                { QStringLiteral("id"), id },
                { QStringLiteral("preview"), preview },
                { QStringLiteral("isImage"), isImage },
            });
            count++;
        }

        m_items = result;
        emit itemsChanged();

        QDir().mkpath(m_imageCacheDir);
        QFile::setPermissions(m_imageCacheDir, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
        for (const auto& entry : std::as_const(m_items)) {
            const auto map = entry.toMap();
            if (!map.value(QStringLiteral("isImage")).toBool())
                continue;
            const int id = map.value(QStringLiteral("id")).toInt();
            const QString outPath =
                m_imageCacheDir + QStringLiteral("/") + QString::number(id) + QStringLiteral(".png");
            if (isImageCached(id)) {
                emit imageReady(id, outPath);
                continue;
            }
            decodeImage(id, outPath);
        }
    });

    connect(proc, &QProcess::started, this, [this] {
        setAvailable(true);
    });

    connect(proc, &QProcess::errorOccurred, this, [this, release](QProcess::ProcessError err) {
        qCWarning(lcClipboard) << "cliphist list process error:" << err;
        if (err == QProcess::FailedToStart) {
            setAvailable(false);
            release();
        }
    });

    proc->start();
}

void ClipboardManager::decodeImage(int id, const QString& outPath) {
    if (isImageCached(id)) {
        emit imageReady(id, outPath);
        return;
    }

    if (m_activeDecodes.contains(id)) {
        return;
    }

    const QFileInfo fi(outPath);
    QDir dir(fi.absolutePath());
    if (!dir.exists() && !dir.mkpath(QStringLiteral("."))) {
        qCWarning(lcClipboard) << "Failed to create cache directory:" << dir.absolutePath();
        return;
    }
    QFile::setPermissions(dir.absolutePath(), QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);

    m_activeDecodes.insert(id);

    auto* proc = new QProcess(this);
    proc->setProgram(QStringLiteral("cliphist"));
    proc->setArguments({ QStringLiteral("decode"), QString::number(id) });

    connect(proc, &QProcess::finished, this, [this, proc, outPath, id](int exitCode, QProcess::ExitStatus) {
        m_activeDecodes.remove(id);
        if (exitCode != 0) {
            qCWarning(lcClipboard) << "cliphist decode failed for id" << id;
            proc->deleteLater();
            return;
        }

        const auto data = proc->readAllStandardOutput();
        proc->deleteLater();

        if (data.isEmpty()) {
            qCWarning(lcClipboard) << "cliphist decode produced empty output for id" << id;
            return;
        }

        QSaveFile f(outPath);
        if (!f.open(QIODevice::WriteOnly)) {
            qCWarning(lcClipboard) << "Failed to write decoded clipboard image to:" << outPath;
            return;
        }
        f.setPermissions(QFile::ReadOwner | QFile::WriteOwner);
        f.write(data);
        if (!f.commit()) {
            qCWarning(lcClipboard) << "Failed to commit decoded clipboard image to:" << outPath;
            return;
        }

        emit imageReady(id, outPath);
    });

    connect(proc, &QProcess::errorOccurred, this, [this, proc, id](QProcess::ProcessError err) {
        m_activeDecodes.remove(id);
        qCWarning(lcClipboard) << "cliphist decode process error for id" << id << ":" << err;
        proc->deleteLater();
    });

    proc->start();
}

void ClipboardManager::clearHistory() {
    m_activeDecodes.clear();

    if (m_listProc && m_listProc->state() != QProcess::NotRunning) {
        m_listProc->kill();
        m_listProc->waitForFinished(200);
        m_listProc->deleteLater();
        m_listProc = nullptr;
    }

    if (m_wipeProc && m_wipeProc->state() != QProcess::NotRunning) {
        qCWarning(lcClipboard) << "cliphist wipe already in progress";
        return;
    }

    auto* wipeProc = new QProcess(this);
    wipeProc->setProgram(QStringLiteral("cliphist"));
    wipeProc->setArguments({ QStringLiteral("wipe") });
    m_wipeProc = wipeProc;

    const auto settle = [this, wipeProc](bool success, const QString& reason) {
        if (m_wipeProc != wipeProc) {
            return;
        }
        m_wipeProc = nullptr;
        wipeProc->deleteLater();

        if (!success) {
            qCWarning(lcClipboard) << "cliphist wipe" << reason;
            reload();
            emit clearHistoryFinished(false);
            return;
        }

        m_items.clear();
        emit itemsChanged();

        QDir cacheDir(m_imageCacheDir);
        if (cacheDir.exists() && !cacheDir.removeRecursively()) {
            qCWarning(lcClipboard) << "Failed to clear clipboard image cache:" << m_imageCacheDir;
        }
        if (!QDir().mkpath(m_imageCacheDir)) {
            qCWarning(lcClipboard) << "Failed to recreate clipboard image cache directory:" << m_imageCacheDir;
        } else {
            QFile::setPermissions(m_imageCacheDir, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
        }

        emit clearHistoryFinished(true);
    };

    connect(wipeProc, &QProcess::finished, this, [settle](int exitCode, QProcess::ExitStatus exitStatus) {
        const bool success = (exitStatus == QProcess::NormalExit && exitCode == 0);
        settle(success, success ? QString() : QStringLiteral("failed with exit code %1").arg(exitCode));
    });

    connect(wipeProc, &QProcess::errorOccurred, this, [settle](QProcess::ProcessError err) {
        if (err != QProcess::FailedToStart) {
            return;
        }
        settle(false, QStringLiteral("could not be started"));
    });

    wipeProc->start();
}

} // namespace caelestia::services
