#include "libraryview.h"

#include "library.h"

#include <QQmlEngine>
#include <QRandomGenerator>

#include <algorithm>

namespace {
const QString kWallpapers = QStringLiteral("wallpapers");
const QString kWorkshop = QStringLiteral("workshop");
const QString kThemes = QStringLiteral("themes");
const QString kRices = QStringLiteral("rices");

// Representative hue angle per bucket (scan.go hueToBucketIdx, inverted).
const int kHueDegrees[12] = {5, 40, 70, 100, 130, 160, 190, 220, 250, 280, 305, 337};

QColor hueSatColor(int hue, int sat)
{
    if (hue < 0 || hue > 11)
        return QColor::fromHslF(0.0, 0.0, 0.45);
    const qreal s = qBound(0.2, sat / 100.0, 0.9);
    return QColor::fromHslF(kHueDegrees[hue] / 360.0, s, 0.5);
}

QColor swatchColor(const QStringList &swatches)
{
    const int idx = swatches.size() > 2 ? 2 : 0;
    if (idx < swatches.size()) {
        const QColor c(swatches.at(idx));
        if (c.isValid())
            return c;
    }
    return QColor::fromHslF(0.0, 0.0, 0.5);
}

QString folderOfName(const QString &name)
{
    const int slash = name.lastIndexOf(QLatin1Char('/'));
    return slash >= 0 ? name.left(slash) : QString();
}
}

LibraryView::LibraryView(QObject *parent)
    : QAbstractListModel(parent)
    , m_notifier(new CardSourceNotifier(this))
{
    m_randomSeed = QRandomGenerator::global()->generate();
}

void LibraryView::componentComplete()
{
    if (QQmlEngine *engine = qmlEngine(this)) {
        m_library = engine->singletonInstance<Library *>(
            QStringLiteral("Ryoku.Ryogami"), QStringLiteral("Library"));
    }
    if (!m_library)
        m_library = Library::instance();
    if (m_library) {
        connect(m_library, &Library::changed, this, &LibraryView::onLibraryChanged);
        connect(m_library, &Library::currentChanged, this, [this]() { refilter(); });
    }
    m_ready = true;
    buildRows();
    refilter();
}

void LibraryView::onLibraryChanged(const QString &collection)
{
    if (collection != m_collection)
        return;
    buildRows();
    refilter();
}

CardSourceNotifier *LibraryView::cardNotifier() const
{
    return m_notifier;
}

void LibraryView::setCollection(const QString &value)
{
    if (m_collection == value)
        return;
    m_collection = value;
    Q_EMIT collectionChanged();
    if (m_ready) {
        buildRows();
        refilter();
    }
}

void LibraryView::setTypeFilter(const QString &value)
{
    if (m_typeFilter == value)
        return;
    m_typeFilter = value;
    Q_EMIT typeFilterChanged();
    refilter();
}

void LibraryView::setHueFilter(int value)
{
    if (m_hueFilter == value)
        return;
    m_hueFilter = value;
    Q_EMIT hueFilterChanged();
    refilter();
}

void LibraryView::setFavouritesOnly(bool value)
{
    if (m_favouritesOnly == value)
        return;
    m_favouritesOnly = value;
    Q_EMIT favouritesOnlyChanged();
    refilter();
}

void LibraryView::setQuery(const QString &value)
{
    if (m_query == value)
        return;
    m_query = value;
    m_queryTokens.clear();
    const QStringList raw = value.toLower().split(QLatin1Char(' '), Qt::SkipEmptyParts);
    m_queryTokens = raw;
    Q_EMIT queryChanged();
    refilter();
}

void LibraryView::setTags(const QStringList &value)
{
    if (m_tags == value)
        return;
    m_tags = value;
    Q_EMIT tagsChanged();
    refilter();
}

void LibraryView::setTagsMatchAny(bool value)
{
    if (m_tagsMatchAny == value)
        return;
    m_tagsMatchAny = value;
    Q_EMIT tagsMatchAnyChanged();
    refilter();
}

void LibraryView::setFolder(const QString &value)
{
    if (m_folder == value)
        return;
    m_folder = value;
    Q_EMIT folderChanged();
    refilter();
}

void LibraryView::setSort(const QString &value)
{
    if (m_sort == value)
        return;
    m_sort = value;
    Q_EMIT sortChanged();
    refilter();
}

QVariantList LibraryView::keyOrder() const
{
    QVariantList out;
    out.reserve(m_keyOrder.size());
    for (const QString &k : m_keyOrder)
        out.append(k);
    return out;
}

void LibraryView::setKeyOrder(const QVariantList &value)
{
    QStringList keys;
    keys.reserve(value.size());
    for (const QVariant &v : value) {
        const QString k = v.toString();
        if (!k.isEmpty())
            keys.append(k);
    }
    if (m_keyOrder == keys)
        return;
    m_keyOrder = keys;
    Q_EMIT keyOrderChanged();
    refilter();
}

void LibraryView::setMinWidth(int value)
{
    if (m_minWidth == value)
        return;
    m_minWidth = value;
    Q_EMIT minWidthChanged();
    refilter();
}

void LibraryView::setMinHeight(int value)
{
    if (m_minHeight == value)
        return;
    m_minHeight = value;
    Q_EMIT minHeightChanged();
    refilter();
}

void LibraryView::setMaxWidth(int value)
{
    if (m_maxWidth == value)
        return;
    m_maxWidth = value;
    Q_EMIT maxWidthChanged();
    refilter();
}

void LibraryView::setMaxHeight(int value)
{
    if (m_maxHeight == value)
        return;
    m_maxHeight = value;
    Q_EMIT maxHeightChanged();
    refilter();
}

void LibraryView::setOrientation(const QString &value)
{
    if (m_orientation == value)
        return;
    m_orientation = value;
    Q_EMIT orientationChanged();
    refilter();
}

void LibraryView::buildRows()
{
    QVector<Row> rows;
    if (!m_library) {
        m_rows = std::move(rows);
        return;
    }
    if (m_collection == kWallpapers || m_collection == kWorkshop) {
        const QVector<Entry> &src = m_collection == kWorkshop ? m_library->workshop() : m_library->wallpapers();
        rows.reserve(src.size());
        for (const Entry &e : src) {
            Row r;
            r.key = e.key;
            r.name = e.name;
            r.type = e.type;
            r.thumb = e.thumb;
            r.fullImage = e.type == QLatin1String("static") ? (e.path.isEmpty() ? e.thumb : e.path) : e.thumb;
            r.previewVideo = e.videoPrev;
            r.path = !e.path.isEmpty() ? e.path : (e.type == QLatin1String("video") ? e.videoFile : QString());
            r.tags = e.tags;
            r.folder = folderOfName(e.name);
            r.hue = e.hue;
            r.sat = e.sat;
            r.richness = e.richness;
            r.mtime = e.mtime;
            r.applyCount = e.applyCount;
            r.favourite = e.favourite;
            r.width = e.width;
            r.height = e.height;
            r.downloaded = m_collection == kWorkshop || e.type == QLatin1String("we");
            r.fill = hueSatColor(e.hue, e.sat);
            r.search = (e.name + QLatin1Char(' ') + e.title + QLatin1Char(' ')
                        + e.tags.join(QLatin1Char(' ')) + QLatin1Char(' ') + e.path).toLower();
            rows.append(std::move(r));
        }
    } else if (m_collection == kThemes) {
        const QVector<Theme> &src = m_library->themes();
        rows.reserve(src.size());
        for (const Theme &t : src) {
            Row r;
            r.key = t.id;
            r.name = t.label;
            r.type = QStringLiteral("theme");
            r.thumb = t.preview;
            r.fullImage = t.preview;
            r.folder = QString();
            r.fill = swatchColor(t.swatches);
            r.search = t.label.toLower();
            rows.append(std::move(r));
        }
    } else if (m_collection == kRices) {
        const QVector<Rice> &src = m_library->rices();
        rows.reserve(src.size());
        for (const Rice &rice : src) {
            Row r;
            r.key = rice.slug;
            r.name = rice.name;
            r.type = QStringLiteral("rice");
            r.thumb = rice.preview;
            r.fullImage = rice.preview;
            r.tags = rice.tags;
            r.folder = QString();
            r.active = rice.active;
            r.fill = QColor::fromHslF(0.0, 0.0, 0.5);
            r.search = (rice.name + QLatin1Char(' ') + rice.tags.join(QLatin1Char(' '))
                        + QLatin1Char(' ') + rice.author).toLower();
            rows.append(std::move(r));
        }
    }
    m_rows = std::move(rows);
    m_rowByKey.clear();
    m_rowByKey.reserve(m_rows.size());
    for (int i = 0; i < m_rows.size(); ++i)
        m_rowByKey.insert(m_rows[i].key, i);
}

bool LibraryView::appliedOf(const Row &row) const
{
    if (row.type == QLatin1String("rice"))
        return row.active;
    if (row.type == QLatin1String("theme"))
        return false;
    return m_library && m_library->isApplied(row.key);
}

bool LibraryView::passesFacets(const Row &row) const
{
    // Type, colour, favourites, shape and size describe wallpapers. Rice and theme rows
    // carry none of them, so a facet left set on a wallpaper tab would empty those tabs.
    if (m_collection != kWallpapers && m_collection != kWorkshop)
        return true;
    if (!m_typeFilter.isEmpty() && row.type != m_typeFilter)
        return false;
    if (m_hueFilter != -1 && row.hue != m_hueFilter)
        return false;
    if (m_favouritesOnly && row.favourite != 1)
        return false;
    if (m_orientation == QLatin1String("wide")) {
        if (!(row.width > 0 && row.width >= row.height))
            return false;
    } else if (m_orientation == QLatin1String("tall")) {
        if (!(row.height > 0 && row.height > row.width))
            return false;
    }
    if (m_minWidth > 0 && row.width < m_minWidth)
        return false;
    if (m_minHeight > 0 && row.height < m_minHeight)
        return false;
    if (m_maxWidth > 0 && row.width > m_maxWidth)
        return false;
    if (m_maxHeight > 0 && row.height > m_maxHeight)
        return false;
    return true;
}

bool LibraryView::keep(const Row &row) const
{
    if (!passesFacets(row))
        return false;

    if (m_folder != QLatin1String("*") && (m_collection == kWallpapers || m_collection == kWorkshop)) {
        if (m_folder.isEmpty()) {
            if (!row.folder.isEmpty())
                return false;
        } else if (row.folder != m_folder
                   && !row.folder.startsWith(m_folder + QLatin1Char('/'))) {
            return false;
        }
    }

    if (!m_tags.isEmpty()) {
        bool hasPositive = false;
        bool allMatched = true;
        bool anyMatched = false;
        for (const QString &raw : m_tags) {
            if (raw.startsWith(QLatin1Char('-'))) {
                const QString excl = raw.mid(1);
                if (!excl.isEmpty() && row.tags.contains(excl))
                    return false;
                continue;
            }
            hasPositive = true;
            if (row.tags.contains(raw))
                anyMatched = true;
            else
                allMatched = false;
        }
        if (hasPositive) {
            if (row.tags.isEmpty())
                return false;
            if (!(m_tagsMatchAny ? anyMatched : allMatched))
                return false;
        }
    }

    for (const QString &token : m_queryTokens) {
        if (!row.search.contains(token))
            return false;
    }
    return true;
}

QVariantMap LibraryView::get(int row) const
{
    if (row < 0 || row >= m_view.size())
        return {};
    QVariantMap out;
    const QHash<int, QByteArray> roles = roleNames();
    const QModelIndex idx = index(row);
    for (auto it = roles.constBegin(); it != roles.constEnd(); ++it)
        out.insert(QString::fromLatin1(it.value()), data(idx, it.key()));
    return out;
}

int LibraryView::newestIndex() const
{
    int newest = -1;
    qint64 newestMtime = 0;
    for (int i = 0; i < m_view.size(); ++i) {
        const qint64 mtime = m_rows[m_view[i]].mtime;
        if (newest < 0 || mtime > newestMtime) {
            newest = i;
            newestMtime = mtime;
        }
    }
    return newest;
}

int LibraryView::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : m_view.size();
}

QVariant LibraryView::data(const QModelIndex &index, int role) const
{
    if (index.row() < 0 || index.row() >= m_view.size())
        return {};
    const int sourceRow = m_view[index.row()];
    if (sourceRow < 0 || sourceRow >= m_rows.size())
        return {};
    const Row &r = m_rows[sourceRow];
    switch (role) {
    case KeyRole: return r.key;
    case NameRole: return r.name;
    case TypeRole: return r.type;
    case ThumbRole: return r.thumb;
    case FullImageRole: return r.fullImage;
    case PreviewVideoRole: return r.previewVideo;
    case PathRole: return r.path;
    case TagsRole: return r.tags;
    case FolderRole: return r.folder;
    case HueRole: return r.hue;
    case SatRole: return r.sat;
    case RichnessRole: return r.richness;
    case MtimeRole: return static_cast<qlonglong>(r.mtime);
    case ApplyCountRole: return r.applyCount;
    case FavouriteRole: return r.favourite;
    case WidthRole: return r.width;
    case HeightRole: return r.height;
    case BadgesRole: return cardBadges(index.row());
    case FillRole: return r.fill;
    case AppliedRole: return appliedOf(r);
    default: return {};
    }
}

QHash<int, QByteArray> LibraryView::roleNames() const
{
    return {
        {KeyRole, "key"},
        {NameRole, "name"},
        {TypeRole, "type"},
        {ThumbRole, "thumb"},
        {FullImageRole, "fullImage"},
        {PreviewVideoRole, "previewVideo"},
        {PathRole, "path"},
        {TagsRole, "tags"},
        {FolderRole, "folder"},
        {HueRole, "hue"},
        {SatRole, "sat"},
        {RichnessRole, "richness"},
        {MtimeRole, "mtime"},
        {ApplyCountRole, "applyCount"},
        {FavouriteRole, "favourite"},
        {WidthRole, "width"},
        {HeightRole, "height"},
        {BadgesRole, "badges"},
        {FillRole, "fill"},
        {AppliedRole, "applied"},
    };
}

QString LibraryView::cardKey(int row) const
{
    return row >= 0 && row < m_view.size() ? m_rows[m_view[row]].key : QString();
}

QString LibraryView::cardThumb(int row) const
{
    return row >= 0 && row < m_view.size() ? m_rows[m_view[row]].thumb : QString();
}

QString LibraryView::cardFullImage(int row) const
{
    return row >= 0 && row < m_view.size() ? m_rows[m_view[row]].fullImage : QString();
}

QSizeF LibraryView::cardImageSize(int row) const
{
    if (row < 0 || row >= m_view.size())
        return QSizeF(16, 9);
    const Row &r = m_rows[m_view[row]];
    if (r.width > 0 && r.height > 0)
        return QSizeF(r.width, r.height);
    return QSizeF(16, 9);
}

QColor LibraryView::cardFill(int row) const
{
    return row >= 0 && row < m_view.size() ? m_rows[m_view[row]].fill : QColor();
}

unsigned LibraryView::cardBadges(int row) const
{
    if (row < 0 || row >= m_view.size())
        return CardSource::BadgeNone;
    const Row &r = m_rows[m_view[row]];
    unsigned badges = CardSource::BadgeNone;
    if (r.favourite == 1)
        badges |= CardSource::BadgeFavourite;
    if (r.type == QLatin1String("video"))
        badges |= CardSource::BadgeVideo;
    if (r.type == QLatin1String("we"))
        badges |= CardSource::BadgeScene;
    if (r.downloaded)
        badges |= CardSource::BadgeDownloaded;
    if (appliedOf(r))
        badges |= CardSource::BadgeApplied;
    return badges;
}

QString LibraryView::cardPreviewVideo(int row) const
{
    return row >= 0 && row < m_view.size() ? m_rows[m_view[row]].previewVideo : QString();
}

void LibraryView::refilter()
{
    if (!m_ready)
        return;

    // Every branch ends with a key tiebreak so the order is stable across refilters.
    auto applied = [this](const Row &r) { return appliedOf(r); };
    auto less = [this, &applied](int ia, int ib) -> bool {
        const Row &a = m_rows[ia];
        const Row &b = m_rows[ib];
        const QString &s = m_sort;
        if (s == QLatin1String("date") || s == QLatin1String("newest")) {
            if (a.mtime != b.mtime) return a.mtime > b.mtime;
        } else if (s == QLatin1String("recent")) {
            if (a.applyCount != b.applyCount) return a.applyCount > b.applyCount;
            if (a.mtime != b.mtime) return a.mtime > b.mtime;
        } else if (s == QLatin1String("applied")) {
            const bool aa = applied(a), ba = applied(b);
            if (aa != ba) return aa;
            if (a.applyCount != b.applyCount) return a.applyCount > b.applyCount;
            if (a.mtime != b.mtime) return a.mtime > b.mtime;
        } else if (s == QLatin1String("pop") || s == QLatin1String("popular")) {
            const double pa = a.sat - a.richness / 15.0;
            const double pb = b.sat - b.richness / 15.0;
            if (pa != pb) return pa > pb;
        } else if (s == QLatin1String("richness")) {
            if (a.richness != b.richness) return a.richness > b.richness;
            if (a.sat != b.sat) return a.sat > b.sat;
        } else if (s == QLatin1String("minimalist")) {
            if (a.richness != b.richness) return a.richness < b.richness;
            if (a.sat != b.sat) return a.sat > b.sat;
        } else if (s == QLatin1String("res")) {
            const qint64 areaA = qint64(a.width) * a.height;
            const qint64 areaB = qint64(b.width) * b.height;
            if (areaA != areaB) return areaA > areaB;
            if (a.width != b.width) return a.width > b.width;
            if (a.mtime != b.mtime) return a.mtime > b.mtime;
        } else if (s == QLatin1String("name")) {
            const int c = a.name.compare(b.name, Qt::CaseInsensitive);
            if (c != 0) return c < 0;
        } else if (s == QLatin1String("random")) {
            const size_t ha = qHash(a.key, m_randomSeed);
            const size_t hb = qHash(b.key, m_randomSeed);
            if (ha != hb) return ha < hb;
        } else { // color / colour and any unknown key
            const int ha = a.hue == 99 ? 100 : a.hue;
            const int hb = b.hue == 99 ? 100 : b.hue;
            if (ha != hb) return ha < hb;
            if (a.sat != b.sat) return a.sat > b.sat;
        }
        return a.key < b.key;
    };
    auto sigOf = [this, &applied](const Row &r) -> size_t {
        return qHashMulti(0, r.key, r.thumb, r.fullImage, r.previewVideo, r.favourite,
                          int(applied(r)), quint64(r.fill.rgba()), r.type);
    };

    QVector<int> newView;
    if (!m_keyOrder.isEmpty()) {
        // Semantic search fixes membership and order: keep the given keys the facets admit, unsorted.
        newView.reserve(m_keyOrder.size());
        for (const QString &k : m_keyOrder) {
            const auto it = m_rowByKey.constFind(k);
            if (it != m_rowByKey.constEnd() && passesFacets(m_rows[it.value()]))
                newView.append(it.value());
        }
    } else {
        newView.reserve(m_rows.size());
        for (int i = 0; i < m_rows.size(); ++i) {
            if (keep(m_rows[i]))
                newView.append(i);
        }
        std::sort(newView.begin(), newView.end(), less);
    }

    QVector<QString> newKeys;
    QVector<size_t> newSig;
    newKeys.reserve(newView.size());
    newSig.reserve(newView.size());
    for (int idx : newView) {
        newKeys.append(m_rows[idx].key);
        newSig.append(sigOf(m_rows[idx]));
    }

    QHash<QString, size_t> oldSigByKey;
    oldSigByKey.reserve(m_keys.size());
    for (int i = 0; i < m_keys.size(); ++i)
        oldSigByKey.insert(m_keys[i], m_sig[i]);

    // A library refresh replaces m_rows before this method runs. Re-anchor the current
    // model rows by stable key so incremental remove/insert signals expose valid data.
    if (!m_keys.isEmpty()) {
        m_view.clear();
        m_view.reserve(m_keys.size());
        for (const QString &key : m_keys)
            m_view.append(m_rowByKey.value(key, -1));
    }

    const int oldCount = m_keys.size();
    const bool sameKeys = (newKeys == m_keys);
    bool structural = false;
    bool didReset = false;

    if (sameKeys) {
        if (newSig == m_sig) {
            // Nothing visible changed: refresh the index mapping without touching the model.
            m_view = newView;
            return;
        }
        m_view = newView;
    } else {
        structural = true;
        didReset = applyIncremental(newView, newKeys);
    }

    m_keys = newKeys;
    m_sig = newSig;
    m_keyToView.clear();
    m_keyToView.reserve(m_keys.size());
    for (int i = 0; i < m_keys.size(); ++i)
        m_keyToView.insert(m_keys[i], i);

    // Repaint only surviving rows whose render signature moved.
    bool metadataChanged = false;
    if (!didReset) {
        int runStart = -1;
        for (int i = 0; i <= m_view.size(); ++i) {
            const bool changed = i < m_view.size()
                && oldSigByKey.contains(m_keys[i])
                && oldSigByKey.value(m_keys[i]) != m_sig[i];
            if (changed) {
                if (runStart < 0)
                    runStart = i;
            } else if (runStart >= 0) {
                Q_EMIT dataChanged(index(runStart), index(i - 1));
                metadataChanged = true;
                runStart = -1;
            }
        }
    }

    if (structural) {
        ++m_generation;
        Q_EMIT generationChanged();
    }
    if (m_view.size() != oldCount)
        Q_EMIT countChanged();
    if (structural || metadataChanged)
        Q_EMIT m_notifier->cardsChanged();
}

bool LibraryView::applyIncremental(const QVector<int> &newView, const QVector<QString> &newKeys)
{
    QHash<QString, int> newPos;
    newPos.reserve(newKeys.size());
    for (int i = 0; i < newKeys.size(); ++i)
        newPos.insert(newKeys[i], i);

    // Shared rows kept their relative order: edit in place with inserts and removes, else reset.
    int last = -1;
    for (const QString &k : m_keys) {
        const auto it = newPos.constFind(k);
        if (it == newPos.constEnd())
            continue;
        if (it.value() <= last) {
            beginResetModel();
            m_view = newView;
            endResetModel();
            return true;
        }
        last = it.value();
    }

    int editRuns = 0;
    bool editing = false;
    for (const QString &key : m_keys) {
        const bool remove = !newPos.contains(key);
        if (remove && !editing)
            ++editRuns;
        editing = remove;
    }
    editing = false;
    for (const QString &key : newKeys) {
        const bool insert = !m_keyToView.contains(key);
        if (insert && !editing)
            ++editRuns;
        editing = insert;
    }
    // Hundreds of alternating begin/end pairs cost more than one bounded reset.
    if (editRuns > 64) {
        beginResetModel();
        m_view = newView;
        endResetModel();
        return true;
    }

    QVector<QString> cur = m_keys;
    for (int i = cur.size() - 1; i >= 0;) {
        if (!newPos.contains(cur[i])) {
            int j = i;
            while (j >= 0 && !newPos.contains(cur[j]))
                --j;
            beginRemoveRows(QModelIndex(), j + 1, i);
            cur.remove(j + 1, i - j);
            m_view.remove(j + 1, i - j);
            endRemoveRows();
            i = j;
        } else {
            --i;
        }
    }
    for (int p = 0; p < newKeys.size();) {
        if (p < cur.size() && cur[p] == newKeys[p]) {
            ++p;
            continue;
        }
        int end = p;
        while (end < newKeys.size() && (p >= cur.size() || newKeys[end] != cur[p]))
            ++end;
        beginInsertRows(QModelIndex(), p, end - 1);
        for (int at = p; at < end; ++at) {
            cur.insert(at, newKeys[at]);
            m_view.insert(at, newView[at]);
        }
        endInsertRows();
        p = end;
    }
    m_view = newView;
    return false;
}
