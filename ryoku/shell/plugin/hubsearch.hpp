#pragma once

#include <QHash>
#include <QJSValue>
#include <QObject>
#include <QStringList>
#include <QVector>
#include <qqmlregistration.h>

class HubSearchIndex : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(qulonglong revision READ revision NOTIFY revisionChanged)

public:
    explicit HubSearchIndex(QObject *parent = nullptr);

    qulonglong revision() const { return m_revision; }

    Q_INVOKABLE void setEntries(QJSValue entries);
    Q_INVOKABLE void setSynonyms(QJSValue map);
    Q_INVOKABLE QJSValue query(QString q, int limit) const;

signals:
    void revisionChanged();

private:
    struct Entry {
        QJSValue value;
        QStringList fullWords;
        QStringList labelWords;
        QStringList keywordWords;
        bool isPage = false;
    };

    struct ScoredEntry {
        qsizetype index = 0;
        double score = 0.0;
    };

    static QStringList tokenize(const QString &text);
    static int editDistance(const QString &a, const QString &b, int maximum);
    static double wordScore(const QString &word, const QStringList &textWords);
    static double searchScore(const QList<QStringList> &groups, const QStringList &textWords);
    QList<QStringList> expandQuery(const QString &query) const;

    QVector<Entry> m_entries;
    QHash<QString, QStringList> m_synonyms;
    qulonglong m_revision = 0;
};
