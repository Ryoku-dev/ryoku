#include "fuzzyranker.hpp"

#include <QJSEngine>
#include <QQmlEngine>
#include <QStringList>
#include <QVector>

#include <algorithm>
#include <utility>

namespace {

struct RankedEntry {
    QJSValue value;
    QString name;
    QStringList fields;
    double usage = 0.0;
    int score = 99;
};

QString stringIfTruthy(const QJSValue &value)
{
    return value.toBool() ? value.toString() : QString();
}

RankedEntry normalize(const QJSValue &entry, bool includeFields = true)
{
    RankedEntry out;
    out.value = entry;

    const QString rawName = stringIfTruthy(entry.property(QStringLiteral("name")));
    out.name = rawName.toLower();
    if (!includeFields)
        return out;
    if (!rawName.isEmpty())
        out.fields.append(out.name);

    const QString genericName = stringIfTruthy(entry.property(QStringLiteral("genericName")));
    if (!genericName.isEmpty())
        out.fields.append(genericName.toLower());

    const QJSValue keywords = entry.property(QStringLiteral("keywords"));
    if (keywords.toBool()) {
        const quint32 length = keywords.property(QStringLiteral("length")).toUInt();
        out.fields.reserve(out.fields.size() + int(length));
        for (quint32 i = 0; i < length; ++i)
            out.fields.append(keywords.property(i).toString().toLower());
    }

    return out;
}

bool isSubsequence(const QString &needle, const QString &haystack)
{
    qsizetype j = 0;
    for (qsizetype i = 0; i < haystack.size() && j < needle.size(); ++i) {
        if (haystack.at(i) == needle.at(j))
            ++j;
    }
    return j == needle.size();
}

int entryScore(const RankedEntry &entry, const QString &query)
{
    if (entry.name.startsWith(query))
        return 0;

    int best = 99;
    for (const QString &field : entry.fields) {
        if (field.contains(query)) {
            best = std::min(best, 1);
            continue;
        }
        if (isSubsequence(query, field))
            best = std::min(best, 2);
    }
    return best;
}

double usageFor(const QJSValue &usage, const QJSValue &entry)
{
    if (!usage.toBool() || !entry.toBool())
        return 0.0;
    const QJSValue id = entry.property(QStringLiteral("id"));
    if (!id.toBool())
        return 0.0;
    const QJSValue count = usage.property(id.toString());
    return count.isNumber() ? count.toNumber() : 0.0;
}

bool alphabetical(const RankedEntry &left, const RankedEntry &right)
{
    return QString::localeAwareCompare(left.name, right.name) < 0;
}

QJSValue valuesToArray(QJSEngine *engine, const QVector<RankedEntry> &entries)
{
    if (!engine)
        return QJSValue(QJSValue::UndefinedValue);
    QJSValue out = engine->newArray(quint32(entries.size()));
    for (qsizetype i = 0; i < entries.size(); ++i)
        out.setProperty(quint32(i), entries.at(i).value);
    return out;
}

}

FuzzyRanker::FuzzyRanker(QJSEngine *engine, QObject *parent)
    : QObject(parent)
    , m_engine(engine)
{
}

FuzzyRanker *FuzzyRanker::create(QQmlEngine *, QJSEngine *js)
{
    return new FuzzyRanker(js);
}

QJSValue FuzzyRanker::rank(QJSValue entries, QString query, QJSValue usage) const
{
    query = query.trimmed().toLower();
    const bool scoring = !query.isEmpty();
    QVector<RankedEntry> ranked;
    const quint32 length = entries.property(QStringLiteral("length")).toUInt();
    ranked.reserve(int(length));
    for (quint32 i = 0; i < length; ++i) {
        const QJSValue entry = entries.property(i);
        if (entry.property(QStringLiteral("noDisplay")).toBool())
            continue;
        RankedEntry normalized = normalize(entry, scoring);
        if (scoring) {
            normalized.score = entryScore(normalized, query);
            if (normalized.score >= 99)
                continue;
        }
        normalized.usage = usageFor(usage, entry);
        ranked.append(std::move(normalized));
    }

    std::stable_sort(ranked.begin(), ranked.end(), [scoring](const RankedEntry &left, const RankedEntry &right) {
        if (scoring && left.score != right.score)
            return left.score < right.score;
        if (left.usage != right.usage)
            return left.usage > right.usage;
        return alphabetical(left, right);
    });
    return valuesToArray(m_engine, ranked);
}

int FuzzyRanker::score(QJSValue entry, QString query) const
{
    return entryScore(normalize(entry), query);
}

QJSValue FuzzyRanker::highlight(QString name, QString query) const
{
    if (!m_engine)
        return QJSValue(QJSValue::UndefinedValue);

    query = query.toLower();
    if (query.isEmpty())
        return m_engine->newArray();

    const QString haystack = name.toLower();
    QVector<QPair<int, int>> spans;
    spans.reserve(query.size());
    qsizetype j = 0;
    for (qsizetype i = 0; i < haystack.size() && j < query.size(); ++i) {
        if (haystack.at(i) != query.at(j))
            continue;
        const int hit = int(i);
        if (!spans.isEmpty() && hit == spans.last().first + spans.last().second)
            ++spans.last().second;
        else
            spans.append({hit, 1});
        ++j;
    }
    if (j < query.size())
        return m_engine->newArray();

    QJSValue out = m_engine->newArray(quint32(spans.size()));
    for (qsizetype i = 0; i < spans.size(); ++i) {
        QJSValue span = m_engine->newObject();
        span.setProperty(QStringLiteral("start"), spans.at(i).first);
        span.setProperty(QStringLiteral("len"), spans.at(i).second);
        out.setProperty(quint32(i), span);
    }
    return out;
}

QJSValue FuzzyRanker::prefixRank(QJSValue entries, QString query, QString field) const
{
    QVector<RankedEntry> matches;
    const quint32 length = entries.property(QStringLiteral("length")).toUInt();
    matches.reserve(int(length));
    query = query.trimmed().toLower();

    for (quint32 i = 0; i < length; ++i) {
        const QJSValue entry = entries.property(i);
        RankedEntry ranked;
        ranked.value = entry;
        ranked.name = stringIfTruthy(entry.property(field)).toLower();
        if (!query.isEmpty() && !ranked.name.contains(query))
            continue;
        ranked.score = !query.isEmpty() && !ranked.name.startsWith(query) ? 1 : 0;
        matches.append(std::move(ranked));
    }

    std::stable_sort(matches.begin(), matches.end(), [](const RankedEntry &left, const RankedEntry &right) {
        if (left.score != right.score)
            return left.score < right.score;
        return alphabetical(left, right);
    });
    return valuesToArray(m_engine, matches);
}
