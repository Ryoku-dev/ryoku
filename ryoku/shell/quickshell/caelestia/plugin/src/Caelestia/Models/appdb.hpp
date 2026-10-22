#pragma once

#include <qhash.h>
#include <qobject.h>
#include <qqmlintegration.h>
#include <qqmllist.h>
#include <qregularexpression.h>
#include <qtimer.h>

#include <ext/pb_ds/assoc_container.hpp>
#include <ext/pb_ds/tree_policy.hpp>

namespace caelestia::models {

// Sort key: favourites, descending frequency, ascending name.
struct AppRankKey {
    int notFav;  // 0 = fav, 1 = regular
    int negFreq; // Negative frequency for descending order
    QString name;

    bool operator<(const AppRankKey& o) const {
        if (notFav != o.notFav)
            return notFav < o.notFav;
        if (negFreq != o.negFreq)
            return negFreq < o.negFreq;
        return name.localeAwareCompare(o.name) < 0;
    }
};

class AppEntry;

using AppRankTree = __gnu_pbds::tree<AppRankKey, AppEntry*, std::less<AppRankKey>, __gnu_pbds::rb_tree_tag,
    __gnu_pbds::tree_order_statistics_node_update>;

class AppEntry : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_UNCREATABLE("AppEntry instances can only be retrieved from an AppDb")

    Q_PROPERTY(QObject* entry READ entry CONSTANT)

    Q_PROPERTY(quint32 frequency READ frequency NOTIFY frequencyChanged)
    Q_PROPERTY(QString id READ id CONSTANT)
    Q_PROPERTY(QString name READ name NOTIFY nameChanged)
    Q_PROPERTY(QString comment READ comment NOTIFY commentChanged)
    Q_PROPERTY(QString execString READ execString NOTIFY execStringChanged)
    Q_PROPERTY(QString startupClass READ startupClass NOTIFY startupClassChanged)
    Q_PROPERTY(QString genericName READ genericName NOTIFY genericNameChanged)
    Q_PROPERTY(QString categories READ categories NOTIFY categoriesChanged)
    Q_PROPERTY(QString keywords READ keywords NOTIFY keywordsChanged)

public:
    explicit AppEntry(QObject* entry, quint32 frequency, QObject* parent = nullptr);

    [[nodiscard]] QObject* entry() const;

    [[nodiscard]] quint32 frequency() const;
    void setFrequency(quint32 frequency);
    void incrementFrequency();

    [[nodiscard]] QString id() const;
    [[nodiscard]] QString name() const;
    [[nodiscard]] QString comment() const;
    [[nodiscard]] QString execString() const;
    [[nodiscard]] QString startupClass() const;
    [[nodiscard]] QString genericName() const;
    [[nodiscard]] QString categories() const;
    [[nodiscard]] QString keywords() const;

signals:
    void frequencyChanged();
    void nameChanged();
    void commentChanged();
    void execStringChanged();
    void startupClassChanged();
    void genericNameChanged();
    void categoriesChanged();
    void keywordsChanged();
    void removed();

private:
    QObject* m_entry;
    quint32 m_frequency;

    void onEntryDestroyed();
};

class AppDb : public QObject {
    Q_OBJECT
    QML_ELEMENT

    Q_PROPERTY(QString uuid READ uuid CONSTANT)
    Q_PROPERTY(QString path READ path WRITE setPath NOTIFY pathChanged REQUIRED)
    Q_PROPERTY(QObjectList entries READ entries WRITE setEntries NOTIFY entriesChanged REQUIRED)
    Q_PROPERTY(QStringList favouriteApps READ favouriteApps WRITE setFavouriteApps NOTIFY favouriteAppsChanged REQUIRED)
    Q_PROPERTY(QQmlListProperty<caelestia::models::AppEntry> apps READ apps NOTIFY appsChanged)
    Q_PROPERTY(QVariantList alphaApps READ alphaApps NOTIFY appsChanged)

public:
    explicit AppDb(QObject* parent = nullptr);

    [[nodiscard]] QString uuid() const;

    [[nodiscard]] QString path() const;
    void setPath(const QString& path);

    [[nodiscard]] QObjectList entries() const;
    void setEntries(const QObjectList& entries);

    [[nodiscard]] QStringList favouriteApps() const;
    void setFavouriteApps(const QStringList& favApps);

    [[nodiscard]] QQmlListProperty<AppEntry> apps();
    [[nodiscard]] QVariantList alphaApps() const;

    Q_INVOKABLE void incrementFrequency(const QString& id);

signals:
    void pathChanged();
    void entriesChanged();
    void favouriteAppsChanged();
    void appsChanged();

private:
    QTimer* m_timer;

    const QString m_uuid;
    QString m_path;
    QObjectList m_entries;
    QStringList m_favouriteApps;
    QList<QRegularExpression> m_favouriteAppsRegex;
    QHash<QString, AppEntry*> m_apps;
    AppRankTree m_rankTree;
    mutable QList<AppEntry*> m_cachedSorted;
    QVariantList m_cachedAlphaApps;

    QString regexifyString(const QString& original) const;
    void rebuildRankTree();
    [[nodiscard]] AppRankKey makeKey(const AppEntry* app) const;
    QList<AppEntry*> getSortedApps() const;
    bool isFavourite(const AppEntry* app) const;
    quint32 getFrequency(const QString& id) const;
    void updateAppFrequencies();
    void updateApps();
};

} // namespace caelestia::models
