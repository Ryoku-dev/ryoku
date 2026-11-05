#include "library.h"

#include "daemon.h"

#include <QJsonArray>
#include <QJsonObject>
#include <QJsonValue>
#include <QQmlEngine>
#include <QTimer>

namespace {
constexpr int kCachedFlushMs = 60;

const QString kWallpapers = QStringLiteral("wallpapers");
const QString kWorkshop = QStringLiteral("workshop");
const QString kThemes = QStringLiteral("themes");
const QString kRices = QStringLiteral("rices");

QJsonArray asArray(const QJsonValue &result)
{
    if (result.isArray())
        return result.toArray();
    const QJsonObject o = result.toObject();
    for (const char *field : {"results", "items", "workshop"}) {
        const QJsonValue v = o.value(QLatin1String(field));
        if (v.isArray())
            return v.toArray();
    }
    return {};
}
}

Library *Library::s_instance = nullptr;

Library::Library(QObject *parent)
    : QObject(parent)
    , m_catalogs(new Catalogs(this))
    , m_cachedFlush(new QTimer(this))
{
    m_cachedFlush->setSingleShot(true);
    m_cachedFlush->setInterval(kCachedFlushMs);
    connect(m_cachedFlush, &QTimer::timeout, this, &Library::flushCached);
    connect(m_catalogs, &Catalogs::themesChanged, this, [this]() { Q_EMIT changed(kThemes); });
    connect(m_catalogs, &Catalogs::ricesChanged, this, [this]() { Q_EMIT changed(kRices); });
}

Library *Library::create(QQmlEngine *engine, QJSEngine *)
{
    if (!s_instance) {
        s_instance = new Library;
        QJSEngine::setObjectOwnership(s_instance, QJSEngine::CppOwnership);
    }
    if (!s_instance->m_daemon) {
        s_instance->m_daemon = engine->singletonInstance<Daemon *>(
            QStringLiteral("Ryoku.Ryogami"), QStringLiteral("Daemon"));
        if (s_instance->m_daemon) {
            connect(s_instance->m_daemon, &Daemon::reconnected, s_instance, &Library::onReconnected);
            connect(s_instance->m_daemon, &Daemon::event, s_instance, &Library::onEvent);
            if (s_instance->m_daemon->connected())
                s_instance->onReconnected();
        }
        s_instance->m_catalogs->start();
    }
    return s_instance;
}

QVariantMap Library::entry(const QString &collection, const QString &key) const
{
    if (collection == kWallpapers || collection == kWorkshop) {
        const QVector<Entry> &rows = collection == kWorkshop ? m_workshop : m_wallpapers;
        for (const Entry &e : rows) {
            if (e.key == key)
                return e.toVariantMap();
        }
    } else if (collection == kThemes) {
        if (const Theme *t = m_catalogs->theme(key))
            return t->toVariantMap();
    } else if (collection == kRices) {
        if (const Rice *r = m_catalogs->rice(key))
            return r->toVariantMap();
    }
    return {};
}

// fromLocalFile escapes names a bare "file://" prefix would break, such as '#' or '?'.
QUrl Library::fileUrl(const QString &path)
{
    if (path.isEmpty())
        return {};
    if (path.contains(QLatin1String("://")))
        return QUrl(path);
    return QUrl::fromLocalFile(path);
}

QStringList Library::folders(const QString &collection) const
{
    if (collection != kWallpapers && collection != kWorkshop)
        return {};
    const QVector<Entry> &rows = collection == kWorkshop ? m_workshop : m_wallpapers;
    QSet<QString> set;
    for (const Entry &e : rows) {
        const int slash = e.name.lastIndexOf(QLatin1Char('/'));
        QString folder = slash >= 0 ? e.name.left(slash) : QString();
        while (!folder.isEmpty()) {
            set.insert(folder);
            const int cut = folder.lastIndexOf(QLatin1Char('/'));
            folder = cut >= 0 ? folder.left(cut) : QString();
        }
    }
    QStringList out = set.values();
    out.sort();
    return out;
}

QVariantMap Library::tagHistogram(const QString &collection) const
{
    QHash<QString, int> counts;
    auto tally = [&counts](const QStringList &tags) {
        for (const QString &t : tags)
            ++counts[t];
    };
    if (collection == kWallpapers || collection == kWorkshop) {
        const QVector<Entry> &rows = collection == kWorkshop ? m_workshop : m_wallpapers;
        for (const Entry &e : rows)
            tally(e.tags);
    } else if (collection == kRices) {
        for (const Rice &r : m_catalogs->rices())
            tally(r.tags);
    }
    QVariantMap out;
    for (auto it = counts.constBegin(); it != counts.constEnd(); ++it)
        out.insert(it.key(), it.value());
    return out;
}

void Library::onReconnected()
{
    fetchWallpapers();
    fetchWorkshop();
    refreshOutputs();
}

void Library::onEvent(const QString &name, const QVariantMap &data)
{
    if (name == QLatin1String("ryogami.wall.cached")) {
        const Entry entry = Entry::fromJson(QJsonObject::fromVariantMap(data));
        upsertWallpaper(entry);
        // Scenes show in the Workshop collection too, so a scene the scan just found
        // must reach it as well as a fresh still for one it already holds.
        if (entry.type == QLatin1String("we")) {
            upsertWorkshop(entry);
            m_workshopDirty = true;
        }
        if (!m_cachedFlush->isActive())
            m_cachedFlush->start();
    } else if (name == QLatin1String("ryogami.wall.file_removed")) {
        removeWallpaper(data.value(QStringLiteral("name")).toString(),
                        data.value(QStringLiteral("type")).toString());
    } else if (name == QLatin1String("ryogami.wall.scan_done")) {
        // A picker started mid-scan holds only what the daemon had listed so far, and not
        // every entry a scan settles arrives as a cached event; the finished lists are the
        // truth, so take both whole rather than leave cards missing until a relaunch.
        flushCached();
        fetchWallpapers();
        fetchWorkshop();
    } else if (name == QLatin1String("ryogami.wall.cache")) {
        if (data.value(QStringLiteral("status")).toString() == QLatin1String("ready")) {
            flushCached();
            fetchWallpapers();
            fetchWorkshop();
        }
    } else if (name == QLatin1String("ryogami.workshop.changed")) {
        fetchWorkshop();
    } else if (name == QLatin1String("ryogami.wall.applied")) {
        const QString key = data.value(QStringLiteral("key")).toString();
        if (!key.isEmpty())
            m_appliedKeys.insert(key);
        refreshOutputs();
        Q_EMIT currentChanged();
    }
}

void Library::flushCached()
{
    m_cachedFlush->stop();
    Q_EMIT changed(kWallpapers);
    if (m_workshopDirty) {
        m_workshopDirty = false;
        Q_EMIT changed(kWorkshop);
    }
}

void Library::fetchWallpapers()
{
    if (!m_daemon)
        return;
    m_daemon->call(QStringLiteral("wall.list"), QJsonObject{},
                   [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        const QJsonArray rows = result.toObject().value(QStringLiteral("wallpapers")).toArray();
        QVector<Entry> wallpapers;
        wallpapers.reserve(rows.size());
        for (const QJsonValue &v : rows)
            wallpapers.append(Entry::fromJson(v.toObject()));
        m_wallpapers = std::move(wallpapers);
        rebuildWallIndex();
        if (!m_rawOutputs.isEmpty())
            assembleOutputs();
        Q_EMIT changed(kWallpapers);
    });
}

void Library::fetchWorkshop()
{
    if (!m_daemon)
        return;
    m_daemon->call(QStringLiteral("workshop.list"), QJsonObject{},
                   [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        const QJsonArray rows = asArray(result);
        QVector<Entry> workshop;
        workshop.reserve(rows.size());
        for (const QJsonValue &v : rows)
            workshop.append(Entry::fromJson(v.toObject()));
        m_workshop = std::move(workshop);
        Q_EMIT changed(kWorkshop);
    });
}

void Library::refreshOutputs()
{
    if (!m_daemon)
        return;
    m_daemon->call(QStringLiteral("wall.outputs"), QJsonObject{},
                   [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_rawOutputs = result.toObject().value(QStringLiteral("outputs")).toObject().toVariantMap();
        assembleOutputs();
    });
    m_daemon->call(QStringLiteral("wm.focusedOutput"), QJsonObject{},
                   [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        const QString focused = result.toObject().value(QStringLiteral("name")).toString();
        if (focused == m_focusedOutput)
            return;
        m_focusedOutput = focused;
        assembleOutputs();
    });
}

void Library::assembleOutputs()
{
    QVariantList list;
    QVariantMap byOutput;
    QSet<QString> applied;
    QStringList names = m_rawOutputs.keys();
    names.sort();
    for (const QString &name : names) {
        const QVariantMap out = m_rawOutputs.value(name).toMap();
        const QString type = out.value(QStringLiteral("type")).toString();
        const QString weId = out.value(QStringLiteral("we_id")).toString();
        const QString path = out.value(QStringLiteral("path")).toString();
        QString key;
        if (!weId.isEmpty()) {
            for (const Entry &e : m_workshop) {
                if (e.weId == weId) { key = e.key; break; }
            }
        } else if (!path.isEmpty()) {
            for (const Entry &e : m_wallpapers) {
                if (e.path == path || e.videoFile == path) { key = e.key; break; }
            }
        }
        if (!key.isEmpty()) {
            byOutput.insert(name, key);
            applied.insert(key);
        }
        // Prefer the logical size; either may be absent until the daemon reports it.
        const QVariant logicalW = out.value(QStringLiteral("logical_width"));
        const QVariant logicalH = out.value(QStringLiteral("logical_height"));
        const QVariant width = logicalW.isValid() ? logicalW : out.value(QStringLiteral("width"), 0);
        const QVariant height = logicalH.isValid() ? logicalH : out.value(QStringLiteral("height"), 0);
        QVariantMap current{
            {QStringLiteral("type"), type},
            {QStringLiteral("key"), key},
            {QStringLiteral("path"), path},
            {QStringLiteral("mute"), out.value(QStringLiteral("mute"))},
            {QStringLiteral("volume"), out.value(QStringLiteral("volume"))},
            {QStringLiteral("paused"), out.value(QStringLiteral("paused"), false)},
            {QStringLiteral("manualPaused"), out.value(QStringLiteral("manual_paused"), false)},
        };
        list.append(QVariantMap{
            {QStringLiteral("name"), name},
            {QStringLiteral("width"), width},
            {QStringLiteral("height"), height},
            {QStringLiteral("focused"), name == m_focusedOutput},
            {QStringLiteral("current"), current},
        });
    }
    m_outputs = list;
    Q_EMIT outputsChanged();
    m_currentByOutput = byOutput;
    m_appliedKeys = applied;
    Q_EMIT currentChanged();
}

void Library::upsertWallpaper(const Entry &entry)
{
    const auto it = m_wallIndex.constFind(entry.key);
    if (it != m_wallIndex.constEnd())
        m_wallpapers[it.value()] = entry;
    else {
        m_wallIndex.insert(entry.key, m_wallpapers.size());
        m_wallpapers.append(entry);
    }
}

void Library::upsertWorkshop(const Entry &entry)
{
    for (Entry &w : m_workshop) {
        if (w.key == entry.key) {
            w = entry;
            return;
        }
    }
    m_workshop.append(entry);
}

void Library::removeWallpaper(const QString &name, const QString &type)
{
    for (int i = 0; i < m_wallpapers.size(); ++i) {
        const Entry &e = m_wallpapers[i];
        if (e.name == name && (type.isEmpty() || e.type == type)) {
            m_wallpapers.remove(i);
            rebuildWallIndex();
            Q_EMIT changed(kWallpapers);
            return;
        }
    }
}

void Library::rebuildWallIndex()
{
    m_wallIndex.clear();
    m_wallIndex.reserve(m_wallpapers.size());
    for (int i = 0; i < m_wallpapers.size(); ++i)
        m_wallIndex.insert(m_wallpapers[i].key, i);
}
