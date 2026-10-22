#include "appdb.hpp"

#include <qloggingcategory.h>
#include <qsqldatabase.h>
#include <qsqlquery.h>
#include <quuid.h>

Q_LOGGING_CATEGORY(lcAppDb, "caelestia.appdb", QtInfoMsg)

namespace caelestia::models {

AppEntry::AppEntry(QObject* entry, unsigned int frequency, QObject* parent)
    : QObject(parent)
    , m_entry(entry)
    , m_frequency(frequency) {
    const auto mo = m_entry->metaObject();
    const auto tmo = &AppEntry::staticMetaObject;

    for (const auto& prop :
        { "name", "comment", "execString", "startupClass", "genericName", "categories", "keywords" }) {
        const auto metaProp = mo->property(mo->indexOfProperty(prop));
        const auto thisMetaProp = tmo->property(tmo->indexOfProperty(prop));
        QObject::connect(m_entry, metaProp.notifySignal(), this, thisMetaProp.notifySignal());
    }

    QObject::connect(m_entry, &QObject::destroyed, this, &AppEntry::onEntryDestroyed);
}

void AppEntry::onEntryDestroyed() {
    m_entry = nullptr;
    emit removed();
    deleteLater();
}

QObject* AppEntry::entry() const {
    return m_entry;
}

quint32 AppEntry::frequency() const {
    return m_frequency;
}

void AppEntry::setFrequency(unsigned int frequency) {
    if (m_frequency != frequency) {
        m_frequency = frequency;
        emit frequencyChanged();
    }
}

void AppEntry::incrementFrequency() {
    m_frequency++;
    emit frequencyChanged();
}

QString AppEntry::id() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("id").toString();
}

QString AppEntry::name() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("name").toString();
}

QString AppEntry::comment() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("comment").toString();
}

QString AppEntry::execString() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("execString").toString();
}

QString AppEntry::startupClass() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("startupClass").toString();
}

QString AppEntry::genericName() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("genericName").toString();
}

QString AppEntry::categories() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("categories").toStringList().join(QStringLiteral(" "));
}

QString AppEntry::keywords() const {
    if (!m_entry) {
        return {};
    }
    return m_entry->property("keywords").toStringList().join(QStringLiteral(" "));
}

AppDb::AppDb(QObject* parent)
    : QObject(parent)
    , m_timer(new QTimer(this))
    , m_uuid(QUuid::createUuid().toString()) {
    m_timer->setSingleShot(true);
    m_timer->setInterval(300);
    QObject::connect(m_timer, &QTimer::timeout, this, &AppDb::updateApps);

    auto db = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), m_uuid);
    db.setDatabaseName(QStringLiteral(":memory:"));
    db.open();

    QSqlQuery query(db);
    query.exec(QStringLiteral("CREATE TABLE IF NOT EXISTS frequencies (id TEXT PRIMARY KEY, frequency INTEGER)"));
}

QString AppDb::uuid() const {
    return m_uuid;
}

QString AppDb::path() const {
    return m_path;
}

void AppDb::setPath(const QString& path) {
    auto newPath = path.isEmpty() ? QStringLiteral(":memory:") : path;

    if (m_path == newPath) {
        return;
    }

    m_path = newPath;
    emit pathChanged();

    auto db = QSqlDatabase::database(m_uuid, false);
    db.close();
    db.setDatabaseName(newPath);
    db.open();

    QSqlQuery query(db);
    query.exec(QStringLiteral("CREATE TABLE IF NOT EXISTS frequencies (id TEXT PRIMARY KEY, frequency INTEGER)"));

    updateAppFrequencies();
}

QObjectList AppDb::entries() const {
    return m_entries;
}

void AppDb::setEntries(const QObjectList& entries) {
    if (m_entries == entries) {
        return;
    }

    m_entries = entries;
    emit entriesChanged();

    m_timer->start();
}

QStringList AppDb::favouriteApps() const {
    return m_favouriteApps;
}

void AppDb::setFavouriteApps(const QStringList& favApps) {
    if (m_favouriteApps == favApps) {
        return;
    }

    m_favouriteApps = favApps;
    emit favouriteAppsChanged();
    m_favouriteAppsRegex.clear();
    m_favouriteAppsRegex.reserve(m_favouriteApps.size());
    for (const QString& item : std::as_const(m_favouriteApps)) {
        const QRegularExpression re(regexifyString(item));
        if (re.isValid()) {
            m_favouriteAppsRegex << re;
        } else {
            qCWarning(lcAppDb) << "setFavouriteApps: regular expression is not valid:" << re.pattern();
        }
    }

    emit appsChanged();
}

QString AppDb::regexifyString(const QString& original) const {
    if (original.startsWith(QLatin1Char('^')) && original.endsWith(QLatin1Char('$')))
        return original;

    const QString escaped = QRegularExpression::escape(original);
    return QStringLiteral("^%1$").arg(escaped);
}

QQmlListProperty<AppEntry> AppDb::apps() {
    auto sorted = getSortedApps();
    m_cachedSorted = std::move(sorted);
    return QQmlListProperty<AppEntry>(this, &m_cachedSorted);
}

QVariantList AppDb::alphaApps() const {
    return m_cachedAlphaApps;
}

void AppDb::incrementFrequency(const QString& id) {
    auto db = QSqlDatabase::database(m_uuid);
    QSqlQuery query(db);

    query.prepare(QStringLiteral("INSERT INTO frequencies (id, frequency) "
                                 "VALUES (:id, 1) "
                                 "ON CONFLICT (id) DO UPDATE SET frequency = frequency + 1"));
    query.bindValue(QStringLiteral(":id"), id);
    query.exec();

    auto* app = m_apps.value(id);
    if (app) {
        m_rankTree.erase(makeKey(app));
        app->incrementFrequency();
        m_rankTree.insert({ makeKey(app), app });
        emit appsChanged();
    } else {
        qCWarning(lcAppDb) << "incrementFrequency: could not find app with id" << id;
    }
}

AppRankKey AppDb::makeKey(const AppEntry* app) const {
    return AppRankKey{
        isFavourite(app) ? 0 : 1,
        -static_cast<int>(app->frequency()),
        app->name(),
    };
}

void AppDb::rebuildRankTree() {
    m_rankTree.clear();
    for (auto* app : std::as_const(m_apps)) {
        m_rankTree.insert({ makeKey(app), app });
    }
}

QList<AppEntry*> AppDb::getSortedApps() const {
    QList<AppEntry*> result;
    result.reserve(static_cast<qsizetype>(m_rankTree.size()));
    for (const auto& [key, entry] : m_rankTree) {
        result.append(entry);
    }
    return result;
}

bool AppDb::isFavourite(const AppEntry* app) const {
    for (const QRegularExpression& re : m_favouriteAppsRegex) {
        if (re.match(app->id()).hasMatch()) {
            return true;
        }
    }
    return false;
}

quint32 AppDb::getFrequency(const QString& id) const {
    auto db = QSqlDatabase::database(m_uuid);
    QSqlQuery query(db);

    query.prepare(QStringLiteral("SELECT frequency FROM frequencies WHERE id = :id"));
    query.bindValue(QStringLiteral(":id"), id);

    if (query.exec() && query.next()) {
        return query.value(0).toUInt();
    }

    return 0;
}

void AppDb::updateAppFrequencies() {
    for (auto* app : std::as_const(m_apps)) {
        app->setFrequency(getFrequency(app->id()));
    }
    rebuildRankTree();
    emit appsChanged();
}

void AppDb::updateApps() {
    bool dirty = false;

    for (const auto& entry : std::as_const(m_entries)) {
        const auto id = entry->property("id").toString();
        if (!m_apps.contains(id)) {
            dirty = true;
            auto* const newEntry = new AppEntry(entry, getFrequency(id), this);
            QObject::connect(newEntry, &AppEntry::removed, this, [id, this]() {
                if (auto* a = m_apps.value(id)) {
                    m_rankTree.erase(makeKey(a));
                }
                if (m_apps.remove(id)) {
                    emit appsChanged();
                }
            });
            m_apps.insert(id, newEntry);
        }
    }

    QSet<QString> newIds;
    for (const auto& entry : std::as_const(m_entries)) {
        newIds.insert(entry->property("id").toString());
    }

    for (auto it = m_apps.begin(); it != m_apps.end();) {
        if (!newIds.contains(it.key())) {
            dirty = true;
            it.value()->deleteLater();
            it = m_apps.erase(it);
        } else {
            ++it;
        }
    }

    if (dirty) {
        rebuildRankTree();

        QList<AppEntry*> alphaList = m_apps.values();
        std::sort(alphaList.begin(), alphaList.end(), [](AppEntry* a, AppEntry* b) {
            return a->name().localeAwareCompare(b->name()) < 0;
        });

        m_cachedAlphaApps.clear();
        m_cachedAlphaApps.reserve(alphaList.size());
        for (auto* a : alphaList) {
            m_cachedAlphaApps.append(QVariant::fromValue(a->entry()));
        }

        emit appsChanged();
    }
}

} // namespace caelestia::models
