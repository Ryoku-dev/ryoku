#include "remoteresults.h"

#include "daemon.h"

#include <QJsonObject>
#include <QJsonValue>
#include <QQmlEngine>
#include <QUrl>

namespace {
const QString kWorkshop = QStringLiteral("steam");

QSizeF parseResolution(const QString &res)
{
    static const QString seps = QStringLiteral("xX\u00d7");
    for (const QChar sep : seps) {
        const int at = res.indexOf(sep);
        if (at <= 0)
            continue;
        bool okW = false, okH = false;
        const int w = res.left(at).trimmed().toInt(&okW);
        const int h = res.mid(at + 1).trimmed().toInt(&okH);
        if (okW && okH && w > 0 && h > 0)
            return QSizeF(w, h);
    }
    return QSizeF(16, 9);
}
}

RemoteResults::RemoteResults(QObject *parent)
    : QAbstractListModel(parent)
    , m_notifier(new CardSourceNotifier(this))
{
}

void RemoteResults::componentComplete()
{
    if (QQmlEngine *engine = qmlEngine(this))
        m_daemon = engine->singletonInstance<Daemon *>(
            QStringLiteral("Ryoku.Ryogami"), QStringLiteral("Daemon"));
    if (!m_daemon)
        m_daemon = Daemon::instance();
    if (m_daemon)
        connect(m_daemon, &Daemon::event, this, &RemoteResults::onEvent);
    m_ready = true;
}

void RemoteResults::setProvider(const QString &value)
{
    if (m_provider == value)
        return;
    m_provider = value;
    // Another provider's rows must not linger under this tab while its search runs.
    clear();
    Q_EMIT providerChanged();
}

void RemoteResults::clear()
{
    ++m_searchGen;
    setLoading(false);
    setError(QString());
    beginResetModel();
    m_rows.clear();
    endResetModel();
    rebuildIdIndex();
    ++m_cardGen;
    // With the old paging kept, the empty grid's infinite scroll would fetch the next page.
    m_page = 1;
    Q_EMIT pageChanged();
    if (m_lastPage != 1) {
        m_lastPage = 1;
        Q_EMIT lastPageChanged();
    }
    Q_EMIT countChanged();
    Q_EMIT m_notifier->cardsChanged();
}

void RemoteResults::setQuery(const QString &value)
{
    if (m_query == value)
        return;
    m_query = value;
    Q_EMIT queryChanged();
}

void RemoteResults::setFilters(const QVariantMap &value)
{
    if (m_filters == value)
        return;
    m_filters = value;
    Q_EMIT filtersChanged();
}

void RemoteResults::setLoading(bool loading)
{
    if (m_loading == loading)
        return;
    m_loading = loading;
    Q_EMIT loadingChanged();
}

void RemoteResults::setError(const QString &error)
{
    if (m_error == error)
        return;
    m_error = error;
    Q_EMIT errorChanged();
}

void RemoteResults::search()
{
    ++m_searchGen;
    m_page = 1;
    Q_EMIT pageChanged();
    setError(QString());
    fetch(1);
}

void RemoteResults::nextPage()
{
    if (m_loading || m_page >= m_lastPage)
        return;
    fetch(m_page + 1);
}

void RemoteResults::fetch(int page)
{
    if (!m_daemon)
        return;
    setLoading(true);
    const bool append = page > 1;
    const quint64 gen = m_searchGen;

    QJsonObject params = QJsonObject::fromVariantMap(m_filters);
    params.insert(QStringLiteral("query"), m_query);
    params.insert(QStringLiteral("page"), page);
    params.insert(QStringLiteral("generation"), static_cast<double>(gen));
    QString method = QStringLiteral("source.search");
    if (m_provider == kWorkshop) {
        method = QStringLiteral("workshop.search");
    } else {
        params.insert(QStringLiteral("source"), m_provider);
    }

    m_daemon->call(method, params, [this, gen, page, append](const QJsonValue &result, const QJsonObject &error) {
        if (gen != m_searchGen)
            return; // a newer search superseded this page
        setLoading(false);
        if (!error.isEmpty()) {
            setError(error.value(QStringLiteral("message")).toString());
            return;
        }
        applyPage(result.toObject().toVariantMap(), page, append);
    });
}

void RemoteResults::applyPage(const QVariantMap &result, int page, bool append)
{
    const QVariantList items = result.value(QStringLiteral("results")).toList();
    QVector<Row> incoming;
    incoming.reserve(items.size());
    for (const QVariant &v : items)
        incoming.append(rowFromItem(v.toMap()));

    if (append) {
        if (!incoming.isEmpty()) {
            const int first = m_rows.size();
            beginInsertRows(QModelIndex(), first, first + incoming.size() - 1);
            m_rows += incoming;
            endInsertRows();
        }
    } else {
        beginResetModel();
        m_rows = std::move(incoming);
        endResetModel();
    }
    rebuildIdIndex();
    ++m_cardGen;

    const QVariant currentPage = result.value(QStringLiteral("currentPage"));
    m_page = currentPage.isValid() ? currentPage.toInt() : page;
    Q_EMIT pageChanged();
    const QVariant lastPageValue = result.value(QStringLiteral("lastPage"));
    const int last = lastPageValue.isValid() ? lastPageValue.toInt() : m_page;
    if (last != m_lastPage) {
        m_lastPage = last;
        Q_EMIT lastPageChanged();
    }
    Q_EMIT countChanged();
    if (append)
        Q_EMIT m_notifier->cardsAppended();
    else
        Q_EMIT m_notifier->cardsChanged();
}

RemoteResults::Row RemoteResults::rowFromItem(const QVariantMap &item)
{
    Row r;
    r.id = item.value(QStringLiteral("id")).toString();
    r.title = item.value(QStringLiteral("title")).toString();
    r.thumb = item.value(QStringLiteral("thumbPath")).toString();
    r.fullUrl = item.value(QStringLiteral("fullUrl")).toString();
    r.trackUrl = item.value(QStringLiteral("trackUrl")).toString();
    r.resolution = item.value(QStringLiteral("resolution")).toString();
    r.fileSize = item.value(QStringLiteral("fileSize")).toLongLong();
    r.purity = item.value(QStringLiteral("purity")).toString();
    r.category = item.value(QStringLiteral("category")).toString();
    r.attribution = item.value(QStringLiteral("attribution")).toString();
    r.attributionUrl = item.value(QStringLiteral("attributionUrl")).toString();
    r.type = item.value(QStringLiteral("type")).toString();
    r.tags = item.value(QStringLiteral("tags")).toStringList();
    r.durationSecs = item.value(QStringLiteral("durationSecs")).toDouble();
    r.downloaded = item.value(QStringLiteral("downloaded")).toBool();
    if (r.downloaded)
        r.downloadStatus = QStringLiteral("done");
    return r;
}

void RemoteResults::download(int row, const QVariantMap &opts)
{
    if (row < 0 || row >= m_rows.size() || !m_daemon)
        return;
    Row &r = m_rows[row];
    r.downloadStatus = QStringLiteral("queued");
    const QModelIndex idx = index(row);
    Q_EMIT dataChanged(idx, idx, {DownloadStatusRole});

    QString method = QStringLiteral("source.download");
    QJsonObject params{{QStringLiteral("id"), r.id}};
    if (m_provider == kWorkshop) {
        method = QStringLiteral("workshop.download");
    } else {
        params.insert(QStringLiteral("source"), m_provider);
        params.insert(QStringLiteral("fullUrl"), r.fullUrl);
        if (!r.trackUrl.isEmpty())
            params.insert(QStringLiteral("trackUrl"), r.trackUrl);
        const QVariant clip = opts.value(QStringLiteral("clip"));
        if (clip.isValid() && !clip.isNull()) {
            const QVariantMap c = clip.toMap();
            params.insert(QStringLiteral("clip"),
                          QJsonObject{{QStringLiteral("start"), c.value(QStringLiteral("start")).toDouble()},
                                      {QStringLiteral("dur"), c.value(QStringLiteral("dur")).toDouble()}});
        }
    }
    const QString id = r.id;
    m_daemon->call(method, params, [this, id](const QJsonValue &result, const QJsonObject &error) {
        const int at = rowOfId(id);
        if (at < 0)
            return;
        if (!error.isEmpty()) {
            m_rows[at].downloadStatus = QStringLiteral("error");
        } else {
            const QJsonObject reply = result.toObject();
            const QString status = reply.value(QStringLiteral("status")).toString();
            if (status == QLatin1String("open_in_steam") || status == QLatin1String("no_steam")) {
                m_rows[at].downloadStatus.clear();
                Q_EMIT openInSteam(id, reply.value(QStringLiteral("url")).toString(),
                                   status == QLatin1String("open_in_steam"));
            } else if (!status.isEmpty() && status != QLatin1String("started")) {
                m_rows[at].downloadStatus = status;
            }
        }
        const QModelIndex idx = index(at);
        Q_EMIT dataChanged(idx, idx, {DownloadStatusRole, DownloadedRole});
    });
}

void RemoteResults::cancelDownload(int row)
{
    if (row < 0 || row >= m_rows.size() || !m_daemon || m_provider == kWorkshop)
        return;
    m_daemon->call(QStringLiteral("source.cancel"), QJsonObject{{QStringLiteral("id"), m_rows[row].id}},
                   [](const QJsonValue &, const QJsonObject &) {});
}

void RemoteResults::preview(int row)
{
    // A clip link plays in the preview itself; there is no still to fetch for it.
    if (row < 0 || row >= m_rows.size() || !m_daemon || !cardPreviewVideo(row).isEmpty())
        return;
    const Row &r = m_rows[row];
    m_daemon->call(QStringLiteral("source.preview"),
                   QJsonObject{{QStringLiteral("source"), m_provider},
                               {QStringLiteral("id"), r.id},
                               {QStringLiteral("fullUrl"), r.fullUrl}}, nullptr);
}

void RemoteResults::onEvent(const QString &name, const QVariantMap &data)
{
    if (name == QLatin1String("ryogami.source.download") || name == QLatin1String("ryogami.workshop.download")) {
        const int at = rowOfId(data.value(QStringLiteral("id")).toString());
        if (at < 0)
            return;
        Row &r = m_rows[at];
        r.downloadStatus = data.value(QStringLiteral("status")).toString();
        if (data.contains(QStringLiteral("progress")))
            r.downloadProgress = data.value(QStringLiteral("progress")).toDouble();
        if (r.downloadStatus == QLatin1String("done")) {
            r.downloaded = true;
            r.downloadProgress = 1.0;
        }
        const QModelIndex idx = index(at);
        Q_EMIT dataChanged(idx, idx, {DownloadStatusRole, DownloadProgressRole, DownloadedRole});
        Q_EMIT m_notifier->cardUpdated(at);
    } else if (name == QLatin1String("ryogami.source.remote_thumb")) {
        const QString source = data.value(QStringLiteral("source")).toString();
        if (!source.isEmpty() && source != m_provider)
            return;
        const int at = rowOfId(data.value(QStringLiteral("id")).toString());
        if (at < 0)
            return;
        const QString path = data.value(QStringLiteral("path")).toString();
        if (!path.isEmpty())
            m_rows[at].thumb = path;
        const QModelIndex idx = index(at);
        Q_EMIT dataChanged(idx, idx, {ThumbRole});
        Q_EMIT m_notifier->cardUpdated(at);
        Q_EMIT thumbArrived(at);
    } else if (name == QLatin1String("ryogami.source.preview_ready")) {
        Q_EMIT previewReady(data.value(QStringLiteral("id")).toString(),
                            data.value(QStringLiteral("path")).toString());
    }
}

void RemoteResults::rebuildIdIndex()
{
    m_idIndex.clear();
    m_idIndex.reserve(m_rows.size());
    for (int i = 0; i < m_rows.size(); ++i)
        m_idIndex.insert(m_rows[i].id, i);
}

QVariantMap RemoteResults::get(int row) const
{
    if (row < 0 || row >= m_rows.size())
        return {};
    QVariantMap out;
    const QHash<int, QByteArray> roles = roleNames();
    const QModelIndex idx = index(row);
    for (auto it = roles.constBegin(); it != roles.constEnd(); ++it)
        out.insert(QString::fromLatin1(it.value()), data(idx, it.key()));
    return out;
}

int RemoteResults::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : m_rows.size();
}

QVariant RemoteResults::data(const QModelIndex &index, int role) const
{
    if (index.row() < 0 || index.row() >= m_rows.size())
        return {};
    const Row &r = m_rows[index.row()];
    switch (role) {
    case IdRole: return r.id;
    case TitleRole: return r.title;
    case ThumbRole: return r.thumb;
    case FullUrlRole: return r.fullUrl;
    case ResolutionRole: return r.resolution;
    case FileSizeRole: return static_cast<qlonglong>(r.fileSize);
    case PurityRole: return r.purity;
    case CategoryRole: return r.category;
    case AttributionRole: return r.attribution;
    case AttributionUrlRole: return r.attributionUrl;
    case DurationSecsRole: return r.durationSecs;
    case DownloadedRole: return r.downloaded;
    case DownloadStatusRole: return r.downloadStatus;
    case DownloadProgressRole: return r.downloadProgress;
    case TagsRole: return r.tags;
    case TypeRole: return r.type;
    case FillRole: return cardFill(index.row());
    default: return {};
    }
}

QHash<int, QByteArray> RemoteResults::roleNames() const
{
    return {
        {IdRole, "id"},
        {TitleRole, "title"},
        {ThumbRole, "thumb"},
        {FullUrlRole, "fullUrl"},
        {ResolutionRole, "resolution"},
        {FileSizeRole, "fileSize"},
        {PurityRole, "purity"},
        {CategoryRole, "category"},
        {AttributionRole, "attribution"},
        {AttributionUrlRole, "attributionUrl"},
        {DurationSecsRole, "durationSecs"},
        {DownloadedRole, "downloaded"},
        {DownloadStatusRole, "downloadStatus"},
        {DownloadProgressRole, "downloadProgress"},
        {TagsRole, "tags"},
        {TypeRole, "type"},
        {FillRole, "fill"},
    };
}

QString RemoteResults::cardKey(int row) const
{
    return row >= 0 && row < m_rows.size()
        ? m_provider + QLatin1Char('/') + m_rows[row].id : QString();
}

int RemoteResults::rowOfKey(const QString &key) const
{
    const QString prefix = m_provider + QLatin1Char('/');
    return key.startsWith(prefix) ? m_idIndex.value(key.mid(prefix.size()), -1) : -1;
}

QString RemoteResults::cardThumb(int row) const
{
    return row >= 0 && row < m_rows.size() ? m_rows[row].thumb : QString();
}

QString RemoteResults::cardFullImage(int row) const
{
    // Nothing local exists before download; the cached thumbnail is the sharpest resident source.
    return row >= 0 && row < m_rows.size() ? m_rows[row].thumb : QString();
}

// A result whose link is the clip itself plays in its card while focused, like a local video.
QString RemoteResults::cardPreviewVideo(int row) const
{
    if (row < 0 || row >= m_rows.size())
        return {};
    const QString url = m_rows[row].fullUrl;
    const QString path = QUrl(url).path().toLower();
    for (const char *ext : {".webm", ".mp4", ".mkv", ".mov"}) {
        if (path.endsWith(QLatin1String(ext)))
            return url;
    }
    return {};
}

QSizeF RemoteResults::cardImageSize(int row) const
{
    if (row < 0 || row >= m_rows.size())
        return QSizeF(16, 9);
    return parseResolution(m_rows[row].resolution);
}

QColor RemoteResults::cardFill(int row) const
{
    Q_UNUSED(row);
    return QColor::fromHslF(0.0, 0.0, 0.28);
}

unsigned RemoteResults::cardBadges(int row) const
{
    if (row < 0 || row >= m_rows.size())
        return CardSource::BadgeNone;
    const Row &r = m_rows[row];
    unsigned badges = CardSource::BadgeNone;
    if (r.type == QLatin1String("video"))
        badges |= CardSource::BadgeVideo;
    if (r.type == QLatin1String("we"))
        badges |= CardSource::BadgeScene;
    if (r.downloaded)
        badges |= CardSource::BadgeDownloaded;
    return badges;
}
