#pragma once

#include "cardsource.h"

#include <QAbstractListModel>
#include <QColor>
#include <QHash>
#include <QString>
#include <QStringList>
#include <QVariantMap>
#include <QVector>
#include <QtQml/qqmlparserstatus.h>
#include <QtQml/qqmlregistration.h>

class Daemon;

class RemoteResults : public QAbstractListModel, public CardSource, public QQmlParserStatus
{
    Q_OBJECT
    QML_ELEMENT
    Q_INTERFACES(QQmlParserStatus)
    Q_PROPERTY(QString provider READ provider WRITE setProvider NOTIFY providerChanged)
    Q_PROPERTY(QString query READ query WRITE setQuery NOTIFY queryChanged)
    Q_PROPERTY(QVariantMap filters READ filters WRITE setFilters NOTIFY filtersChanged)
    Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
    Q_PROPERTY(QString error READ error NOTIFY errorChanged)
    Q_PROPERTY(int page READ page NOTIFY pageChanged)
    Q_PROPERTY(int lastPage READ lastPage NOTIFY lastPageChanged)
    Q_PROPERTY(int count READ count NOTIFY countChanged)

public:
    explicit RemoteResults(QObject *parent = nullptr);

    enum Role {
        IdRole = Qt::UserRole + 1,
        TitleRole,
        ThumbRole,
        FullUrlRole,
        ResolutionRole,
        FileSizeRole,
        PurityRole,
        CategoryRole,
        AttributionRole,
        AttributionUrlRole,
        DurationSecsRole,
        DownloadedRole,
        DownloadStatusRole,
        DownloadProgressRole,
        TagsRole,
        TypeRole,
        FillRole,
    };
    Q_ENUM(Role)

    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    void classBegin() override {}
    void componentComplete() override;

    QString provider() const { return m_provider; }
    void setProvider(const QString &value);
    QString query() const { return m_query; }
    void setQuery(const QString &value);
    QVariantMap filters() const { return m_filters; }
    void setFilters(const QVariantMap &value);
    bool loading() const { return m_loading; }
    QString error() const { return m_error; }
    int page() const { return m_page; }
    int lastPage() const { return m_lastPage; }
    int count() const { return m_rows.size(); }

    Q_INVOKABLE void search();
    Q_INVOKABLE void nextPage();
    // Drops the rows and any search in flight, for a tab with nothing to search yet.
    Q_INVOKABLE void clear();
    // opts.clip = {start, dur} in seconds; the item's own trackUrl is forwarded automatically.
    Q_INVOKABLE void download(int row, const QVariantMap &opts = {});
    // Workshop transfers run in Steam or steamcmd, which the daemon cannot stop midway.
    Q_INVOKABLE void cancelDownload(int row);
    Q_INVOKABLE void preview(int row);
    Q_INVOKABLE QVariantMap get(int row) const;

    int cardCount() const override { return m_rows.size(); }
    QString cardKey(int row) const override;
    QString cardThumb(int row) const override;
    QString cardFullImage(int row) const override;
    QSizeF cardImageSize(int row) const override;
    QColor cardFill(int row) const override;
    unsigned cardBadges(int row) const override;
    QString cardPreviewVideo(int row) const override;
    int rowOfKey(const QString &key) const override;
    quint64 cardGeneration() const override { return m_cardGen; }
    CardSourceNotifier *cardNotifier() const override { return m_notifier; }

Q_SIGNALS:
    void providerChanged();
    void queryChanged();
    void filtersChanged();
    void loadingChanged();
    void errorChanged();
    void pageChanged();
    void lastPageChanged();
    void countChanged();
    void previewReady(const QString &id, const QString &path);
    // steamInstalled false means url is the item's Workshop web page, not a steam:// link.
    void openInSteam(const QString &id, const QString &url, bool steamInstalled);
    // A result's thumbnail finished downloading; the path is unchanged, so a view that loaded early must retry.
    void thumbArrived(int row);

private:
    struct Row {
        QString id;
        QString title;
        QString thumb;
        QString fullUrl;
        QString trackUrl;
        QString resolution;
        qint64 fileSize = 0;
        QString purity;
        QString category;
        QString attribution;
        QString attributionUrl;
        QString type;           // static | video | we
        QStringList tags;
        double durationSecs = 0;
        bool downloaded = false;
        QString downloadStatus; // "" | queued | downloading | done | error | auth_error
        double downloadProgress = 0;
    };

    void fetch(int page);
    void applyPage(const QVariantMap &result, int page, bool append);
    void setLoading(bool loading);
    void setError(const QString &error);
    void onEvent(const QString &name, const QVariantMap &data);
    int rowOfId(const QString &id) const { return m_idIndex.value(id, -1); }
    void rebuildIdIndex();
    static Row rowFromItem(const QVariantMap &item);

    Daemon *m_daemon = nullptr;
    CardSourceNotifier *m_notifier = nullptr;

    QString m_provider;
    QString m_query;
    QVariantMap m_filters;
    bool m_loading = false;
    QString m_error;
    int m_page = 0;
    int m_lastPage = 0;

    QVector<Row> m_rows;
    QHash<QString, int> m_idIndex;
    quint64 m_searchGen = 0;   // stale-page cursor
    quint64 m_cardGen = 0;
    bool m_ready = false;
};
