// SPDX-License-Identifier: GPL-3.0-only
#include "schemeloader.hpp"

#include <qdir.h>
#include <qfile.h>
#include <qfileinfo.h>
#include <qfilesystemwatcher.h>
#include <qjsondocument.h>
#include <qjsonobject.h>
#include <qloggingcategory.h>
#include <qprocess.h>

#include <algorithm>

Q_LOGGING_CATEGORY(lcSchemeLoader, "caelestia.services.schemeloader", QtInfoMsg)

namespace caelestia::services {

SchemeLoader::SchemeLoader(QObject* parent)
    : QObject(parent)
    , m_watcher(new QFileSystemWatcher(this)) {
    const auto stateDir = qEnvironmentVariable("XDG_STATE_HOME", QDir::homePath() + QStringLiteral("/.local/state"));
    m_schemeStatePath = stateDir + QStringLiteral("/caelestia/scheme.json");

    const auto schemeDir = QFileInfo(m_schemeStatePath).absolutePath();
    QDir().mkpath(schemeDir);
    m_watcher->addPath(schemeDir);
    watchSchemeState();

    connect(m_watcher, &QFileSystemWatcher::fileChanged, this, [this](const QString&) {
        loadCurrentScheme();
        // Re-add path because some editors replace files atomically
        watchSchemeState();
    });
    connect(m_watcher, &QFileSystemWatcher::directoryChanged, this, [this](const QString&) {
        if (watchSchemeState()) {
            loadCurrentScheme();
        }
    });

    loadSchemes();
    loadCurrentScheme();
}

SchemeLoader::~SchemeLoader() = default;

QVariantList SchemeLoader::schemes() const {
    return m_schemes;
}

QString SchemeLoader::currentScheme() const {
    return m_currentScheme;
}

QString SchemeLoader::currentVariant() const {
    return m_currentVariant;
}

void SchemeLoader::reloadCurrent() {
    loadCurrentScheme();
}

bool SchemeLoader::watchSchemeState() {
    if (m_watcher->files().contains(m_schemeStatePath) || !QFile::exists(m_schemeStatePath)) {
        return false;
    }
    return m_watcher->addPath(m_schemeStatePath);
}

void SchemeLoader::loadSchemes() {
    auto process = new QProcess(this);
    process->setProgram(QStringLiteral("caelestia"));
    process->setArguments({ QStringLiteral("scheme"), QStringLiteral("list") });

    connect(process, &QProcess::finished, this, [this, process](int exitCode, QProcess::ExitStatus status) {
        process->deleteLater();
        if (status == QProcess::CrashExit || exitCode != 0) {
            qCWarning(lcSchemeLoader) << "Failed to fetch schemes list";
            return;
        }

        const auto response = process->readAllStandardOutput();
        const auto doc = QJsonDocument::fromJson(response);
        if (!doc.isObject())
            return;

        const auto obj = doc.object();
        QVariantList flat;

        for (auto it = obj.begin(); it != obj.end(); ++it) {
            const auto schemeName = it.key();
            const auto flavours = it.value().toObject();
            for (auto fit = flavours.begin(); fit != flavours.end(); ++fit) {
                const auto flavourName = fit.key();
                const auto colours = fit.value().toObject();

                flat.append(
                    QVariantMap{ { QStringLiteral("name"), schemeName }, { QStringLiteral("flavour"), flavourName },
                        { QStringLiteral("colours"), colours.toVariantMap() } });
            }
        }

        std::sort(flat.begin(), flat.end(), [](const QVariant& a, const QVariant& b) {
            const auto ma = a.toMap();
            const auto mb = b.toMap();
            const auto ka =
                ma.value(QStringLiteral("name")).toString() + ma.value(QStringLiteral("flavour")).toString();
            const auto kb =
                mb.value(QStringLiteral("name")).toString() + mb.value(QStringLiteral("flavour")).toString();
            return ka.localeAwareCompare(kb) < 0;
        });

        m_schemes = flat;
        emit schemesChanged();
    });

    process->start();
}

void SchemeLoader::loadCurrentScheme() {
    QFile f(m_schemeStatePath);
    if (!f.exists() || !f.open(QIODevice::ReadOnly)) {
        return;
    }

    const auto doc = QJsonDocument::fromJson(f.readAll());
    if (!doc.isObject())
        return;

    const auto obj = doc.object();
    const auto name = obj.value(QStringLiteral("name")).toString();
    const auto flavour = obj.value(QStringLiteral("flavour")).toString();
    const auto variant = obj.value(QStringLiteral("variant")).toString();

    m_currentScheme = QStringLiteral("%1 %2").arg(name, flavour);
    m_currentVariant = variant;

    emit currentSchemeChanged();
}

} // namespace caelestia::services
