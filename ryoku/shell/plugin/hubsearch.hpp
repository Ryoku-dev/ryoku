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
    // an entry's words, folded for case, and the same words without accents (empty
    // when that changes none of them, which is every word of English text)
    struct Words {
        QStringList words;
        QStringList folded;
    };

    struct Entry {
        QJSValue value;
        Words full;
        Words label;
        Words keywords;
        bool isPage = false;
    };

    // one query word or synonym; folded is empty when it has no accents to drop
    struct Term {
        QString word;
        QString folded;
        bool unspaced = false;
    };
    using Group = QList<Term>;

    struct ScoredEntry {
        qsizetype index = 0;
        double score = 0.0;
    };

    static QString foldCase(const QString &text);
    static QString foldAccents(const QString &word);
    static bool isUnspaced(const QString &word);
    static QStringList tokenize(const QString &text);
    static Words indexWords(const QString &text);
    static Term makeTerm(const QString &word);
    static int editDistance(const QString &a, const QString &b, int maximum);
    static double wordScore(const QString &word, const QStringList &textWords, bool unspaced);
    static double termScore(const Term &term, const Words &text);
    static double searchScore(const QList<Group> &groups, const Words &text);
    QList<Group> expandQuery(const QString &query) const;

    QVector<Entry> m_entries;
    QHash<QString, Group> m_synonyms;
    qulonglong m_revision = 0;
};
