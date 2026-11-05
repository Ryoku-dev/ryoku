#pragma once

#include "cardsource.h"

#include <QAbstractListModel>
#include <QColor>
#include <QHash>
#include <QString>
#include <QStringList>
#include <QVariantMap>
#include <QVector>
#include <QtQml/qqmlregistration.h>
#include <QtQml/qqmlparserstatus.h>

class Library;

class LibraryView : public QAbstractListModel, public CardSource, public QQmlParserStatus
{
    Q_OBJECT
    QML_ELEMENT
    Q_INTERFACES(QQmlParserStatus)
    Q_PROPERTY(QString collection READ collection WRITE setCollection NOTIFY collectionChanged)
    Q_PROPERTY(QString typeFilter READ typeFilter WRITE setTypeFilter NOTIFY typeFilterChanged)
    Q_PROPERTY(int hueFilter READ hueFilter WRITE setHueFilter NOTIFY hueFilterChanged)
    Q_PROPERTY(bool favouritesOnly READ favouritesOnly WRITE setFavouritesOnly NOTIFY favouritesOnlyChanged)
    Q_PROPERTY(QString query READ query WRITE setQuery NOTIFY queryChanged)
    Q_PROPERTY(QStringList tags READ tags WRITE setTags NOTIFY tagsChanged)
    Q_PROPERTY(bool tagsMatchAny READ tagsMatchAny WRITE setTagsMatchAny NOTIFY tagsMatchAnyChanged)
    Q_PROPERTY(QString folder READ folder WRITE setFolder NOTIFY folderChanged)
    Q_PROPERTY(QString sort READ sort WRITE setSort NOTIFY sortChanged)
    Q_PROPERTY(QVariantList keyOrder READ keyOrder WRITE setKeyOrder NOTIFY keyOrderChanged)
    Q_PROPERTY(int minWidth READ minWidth WRITE setMinWidth NOTIFY minWidthChanged)
    Q_PROPERTY(int minHeight READ minHeight WRITE setMinHeight NOTIFY minHeightChanged)
    Q_PROPERTY(int maxWidth READ maxWidth WRITE setMaxWidth NOTIFY maxWidthChanged)
    Q_PROPERTY(int maxHeight READ maxHeight WRITE setMaxHeight NOTIFY maxHeightChanged)
    Q_PROPERTY(QString orientation READ orientation WRITE setOrientation NOTIFY orientationChanged)
    Q_PROPERTY(int count READ count NOTIFY countChanged)
    Q_PROPERTY(qulonglong generation READ generation NOTIFY generationChanged)

public:
    explicit LibraryView(QObject *parent = nullptr);

    enum Role {
        KeyRole = Qt::UserRole + 1,
        NameRole,
        TypeRole,
        ThumbRole,
        FullImageRole,
        PreviewVideoRole,
        PathRole,
        TagsRole,
        FolderRole,
        HueRole,
        SatRole,
        RichnessRole,
        MtimeRole,
        ApplyCountRole,
        FavouriteRole,
        WidthRole,
        HeightRole,
        BadgesRole,
        FillRole,
        AppliedRole,
    };
    Q_ENUM(Role)

    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    void classBegin() override {}
    void componentComplete() override;

    QString collection() const { return m_collection; }
    void setCollection(const QString &value);
    QString typeFilter() const { return m_typeFilter; }
    void setTypeFilter(const QString &value);
    int hueFilter() const { return m_hueFilter; }
    void setHueFilter(int value);
    bool favouritesOnly() const { return m_favouritesOnly; }
    void setFavouritesOnly(bool value);
    QString query() const { return m_query; }
    void setQuery(const QString &value);
    QStringList tags() const { return m_tags; }
    void setTags(const QStringList &value);
    bool tagsMatchAny() const { return m_tagsMatchAny; }
    void setTagsMatchAny(bool value);
    QString folder() const { return m_folder; }
    void setFolder(const QString &value);
    QString sort() const { return m_sort; }
    void setSort(const QString &value);
    QVariantList keyOrder() const;
    void setKeyOrder(const QVariantList &value);
    int minWidth() const { return m_minWidth; }
    void setMinWidth(int value);
    int minHeight() const { return m_minHeight; }
    void setMinHeight(int value);
    int maxWidth() const { return m_maxWidth; }
    void setMaxWidth(int value);
    int maxHeight() const { return m_maxHeight; }
    void setMaxHeight(int value);
    QString orientation() const { return m_orientation; }
    void setOrientation(const QString &value);

    int count() const { return m_view.size(); }
    qulonglong generation() const { return m_generation; }

    Q_INVOKABLE QVariantMap get(int row) const;
    Q_INVOKABLE int indexOfKey(const QString &key) const { return m_keyToView.value(key, -1); }
    Q_INVOKABLE int newestIndex() const;

    int cardCount() const override { return m_view.size(); }
    QString cardKey(int row) const override;
    QString cardThumb(int row) const override;
    QString cardFullImage(int row) const override;
    QSizeF cardImageSize(int row) const override;
    QColor cardFill(int row) const override;
    unsigned cardBadges(int row) const override;
    QString cardPreviewVideo(int row) const override;
    int rowOfKey(const QString &key) const override { return m_keyToView.value(key, -1); }
    quint64 cardGeneration() const override { return m_generation; }
    CardSourceNotifier *cardNotifier() const override;

Q_SIGNALS:
    void collectionChanged();
    void typeFilterChanged();
    void hueFilterChanged();
    void favouritesOnlyChanged();
    void queryChanged();
    void tagsChanged();
    void tagsMatchAnyChanged();
    void folderChanged();
    void sortChanged();
    void keyOrderChanged();
    void minWidthChanged();
    void minHeightChanged();
    void maxWidthChanged();
    void maxHeightChanged();
    void orientationChanged();
    void countChanged();
    void generationChanged();

private:
    struct Row {
        QString key;
        QString name;
        QString type;          // static | video | we | theme | rice
        QString thumb;
        QString fullImage;
        QString previewVideo;
        QString path;
        QString search;        // lowercased name + tags + path
        QStringList tags;
        QString folder;
        int hue = 99;
        int sat = 0;
        int richness = 0;
        qint64 mtime = 0;
        int applyCount = 0;
        int favourite = 0;
        int width = 0;
        int height = 0;
        bool downloaded = false;
        bool active = false;
        QColor fill;
    };

    void buildRows();
    void refilter();
    bool keep(const Row &row) const;
    bool passesFacets(const Row &row) const;
    bool applyIncremental(const QVector<int> &newView, const QVector<QString> &newKeys);
    bool appliedOf(const Row &row) const;
    void onLibraryChanged(const QString &collection);

    Library *m_library = nullptr;
    CardSourceNotifier *m_notifier = nullptr;

    QString m_collection = QStringLiteral("wallpapers");
    QString m_typeFilter;
    int m_hueFilter = -1;
    bool m_favouritesOnly = false;
    QString m_query;
    QStringList m_tags;
    bool m_tagsMatchAny = false;
    QString m_folder = QStringLiteral("*");
    QString m_sort = QStringLiteral("color");
    QStringList m_keyOrder;            // explicit membership+order (semantic search)
    int m_minWidth = 0;
    int m_minHeight = 0;
    int m_maxWidth = 0;
    int m_maxHeight = 0;
    QString m_orientation;             // "" | "wide" | "tall"

    QVector<Row> m_rows;
    QVector<int> m_view;               // indices into m_rows, filtered + sorted
    QVector<QString> m_keys;           // keys aligned with m_view (current)
    QVector<size_t> m_sig;             // render signatures aligned with m_view
    QHash<QString, int> m_keyToView;   // key -> row in m_view
    QHash<QString, int> m_rowByKey;    // key -> row in m_rows
    quint64 m_generation = 0;
    quint32 m_randomSeed = 0;
    QStringList m_queryTokens;
    bool m_ready = false;
};
