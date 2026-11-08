#include "hubsearch.hpp"

#include <QJSEngine>
#include <QJSValueIterator>
#include <QQmlEngine>
#include <QRegularExpression>

#include <algorithm>
#include <utility>

namespace {

QString lowerProperty(const QJSValue &value, const QString &name)
{
    return value.property(name).toString().toLower();
}

// Dense QML arrays use QV4's quicksort, whose equal-score movement is observable.
template<typename RandomAccessIterator, typename LessThan>
void qmlArraySort(RandomAccessIterator start, RandomAccessIterator end, LessThan lessThan)
{
top:
    const int span = int(end - start);
    if (span < 2)
        return;

    --end;
    RandomAccessIterator low = start;
    RandomAccessIterator high = end - 1;
    RandomAccessIterator pivot = start + span / 2;

    if (lessThan(*end, *start))
        std::swap(*end, *start);
    if (span == 2)
        return;

    if (lessThan(*pivot, *start))
        std::swap(*pivot, *start);
    if (lessThan(*end, *pivot))
        std::swap(*end, *pivot);
    if (span == 3)
        return;

    std::swap(*pivot, *end);
    while (low < high) {
        while (low < high && lessThan(*low, *end))
            ++low;
        while (high > low && lessThan(*end, *high))
            --high;
        if (low < high) {
            std::swap(*low, *high);
            ++low;
            --high;
        } else {
            break;
        }
    }

    if (lessThan(*low, *end))
        ++low;
    std::swap(*end, *low);
    qmlArraySort(start, low, lessThan);

    start = low + 1;
    ++end;
    goto top;
}

}

HubSearchIndex::HubSearchIndex(QObject *parent)
    : QObject(parent)
{
}

void HubSearchIndex::setEntries(QJSValue entries)
{
    QVector<Entry> next;
    if (entries.isArray()) {
        const quint32 length = entries.property(QStringLiteral("length")).toUInt();
        next.reserve(length);
        for (quint32 i = 0; i < length; ++i) {
            const QJSValue value = entries.property(i);
            const QString label = lowerProperty(value, QStringLiteral("label"));
            const QString description = lowerProperty(value, QStringLiteral("desc"));
            const QString section = lowerProperty(value, QStringLiteral("sectionName"));
            const QString group = lowerProperty(value, QStringLiteral("group"));
            const QString tab = lowerProperty(value, QStringLiteral("tab"));
            const QString keywords = lowerProperty(value, QStringLiteral("kw"));

            QString full;
            full.reserve(label.size() + description.size() + section.size() + group.size() + tab.size()
                         + keywords.size() + 5);
            full.append(label).append(QLatin1Char(' '));
            full.append(description).append(QLatin1Char(' '));
            full.append(section).append(QLatin1Char(' '));
            full.append(group).append(QLatin1Char(' '));
            full.append(tab).append(QLatin1Char(' '));
            full.append(keywords);

            QString keywordText;
            keywordText.reserve(section.size() + tab.size() + keywords.size() + 2);
            keywordText.append(section).append(QLatin1Char(' '));
            keywordText.append(tab).append(QLatin1Char(' '));
            keywordText.append(keywords);

            Entry entry;
            entry.value = value;
            entry.fullWords = tokenize(full);
            entry.labelWords = tokenize(label);
            entry.keywordWords = tokenize(keywordText);
            entry.isPage = value.property(QStringLiteral("isPage")).toBool();
            next.append(std::move(entry));
        }
    }

    m_entries = std::move(next);
    ++m_revision;
    emit revisionChanged();
}

void HubSearchIndex::setSynonyms(QJSValue map)
{
    QHash<QString, QStringList> next;
    if (map.isObject()) {
        QJSValueIterator it(map);
        while (it.hasNext()) {
            it.next();
            next.insert(it.name(), it.value().toString().split(QRegularExpression(QStringLiteral("\\s+")), Qt::SkipEmptyParts));
        }
    }

    m_synonyms = std::move(next);
    ++m_revision;
    emit revisionChanged();
}

QJSValue HubSearchIndex::query(QString q, int limit) const
{
    QJSEngine *engine = qmlEngine(this);
    if (!engine)
        return {};

    q = q.toLower().trimmed();
    if (q.isEmpty() || limit <= 0)
        return engine->newArray();

    const QList<QStringList> groups = expandQuery(q);
    QVector<ScoredEntry> scored;
    scored.reserve(m_entries.size());
    for (qsizetype i = 0; i < m_entries.size(); ++i) {
        const Entry &entry = m_entries.at(i);
        double score = searchScore(groups, entry.fullWords);
        if (score <= 0.0)
            continue;

        score += 2.0 * searchScore(groups, entry.labelWords);
        const double keywordScore = searchScore(groups, entry.keywordWords);
        score += keywordScore;
        if (entry.isPage)
            score += 300.0 + 3.0 * keywordScore;
        scored.append({i, score});
    }

    qmlArraySort(scored.begin(), scored.end(), [](const ScoredEntry &left, const ScoredEntry &right) {
        return left.score > right.score;
    });

    const qsizetype count = std::min<qsizetype>(scored.size(), limit);
    QJSValue result = engine->newArray(uint(count));
    for (qsizetype i = 0; i < count; ++i)
        result.setProperty(quint32(i), m_entries.at(scored.at(i).index).value);
    return result;
}

QStringList HubSearchIndex::tokenize(const QString &text)
{
    static const QRegularExpression separators(QStringLiteral("[^a-z0-9]+"));
    return text.split(separators, Qt::SkipEmptyParts);
}

int HubSearchIndex::editDistance(const QString &a, const QString &b, int maximum)
{
    const qsizetype aLength = a.size();
    const qsizetype bLength = b.size();
    if (qAbs(aLength - bLength) > maximum)
        return -1;

    QVector<int> previous(bLength + 1);
    QVector<int> current(bLength + 1);
    for (qsizetype j = 0; j <= bLength; ++j)
        previous[j] = int(j);

    for (qsizetype i = 1; i <= aLength; ++i) {
        current[0] = int(i);
        int rowMinimum = int(i);
        for (qsizetype j = 1; j <= bLength; ++j) {
            const int cost = a.at(i - 1) == b.at(j - 1) ? 0 : 1;
            const int value = std::min({previous[j] + 1, current[j - 1] + 1, previous[j - 1] + cost});
            current[j] = value;
            rowMinimum = std::min(rowMinimum, value);
        }
        if (rowMinimum > maximum)
            return -1;
        previous.swap(current);
    }

    return previous[bLength] <= maximum ? previous[bLength] : -1;
}

double HubSearchIndex::wordScore(const QString &word, const QStringList &textWords)
{
    double best = 0.0;
    for (const QString &textWord : textWords) {
        const qsizetype index = textWord.indexOf(word);
        if (index == 0) {
            best = std::max(best, 1200.0);
            continue;
        }
        if (index > 0) {
            best = std::max(best, 1000.0 - double(std::min<qsizetype>(index, 50)));
            continue;
        }
        if (word.size() <= textWord.size()) {
            qsizetype textIndex = 0;
            double score = 0.0;
            int streak = 0;
            bool matched = true;
            for (const QChar character : word) {
                const qsizetype found = textWord.indexOf(character, textIndex);
                if (found < 0) {
                    matched = false;
                    break;
                }
                streak = found == textIndex ? streak + 1 : 0;
                score += 2.0 + streak * 3.0;
                textIndex = found + 1;
            }
            if (matched)
                best = std::max(best, score);
        }
    }

    if (best < 600.0 && word.size() >= 4) {
        const int maximum = word.size() >= 7 ? 2 : 1;
        for (const QString &textWord : textWords) {
            if (textWord.size() < 3)
                continue;
            const int distance = editDistance(word, textWord, maximum);
            if (distance >= 0)
                best = std::max(best, 640.0 - distance * 140.0);
        }
    }
    return best;
}

double HubSearchIndex::searchScore(const QList<QStringList> &groups, const QStringList &textWords)
{
    double total = 0.0;
    int matched = 0;
    for (const QStringList &group : groups) {
        double best = 0.0;
        for (qsizetype i = 0; i < group.size(); ++i) {
            double score = wordScore(group.at(i), textWords);
            if (i > 0)
                score *= 0.85;
            best = std::max(best, score);
        }
        if (best > 0.0) {
            total += best;
            ++matched;
        }
    }

    if (matched == 0 || groups.isEmpty())
        return 0.0;
    return total * (double(matched) / double(groups.size()));
}

QList<QStringList> HubSearchIndex::expandQuery(const QString &query) const
{
    static const QRegularExpression whitespace(QStringLiteral("\\s+"));
    const QStringList words = query.split(whitespace, Qt::SkipEmptyParts);
    QList<QStringList> groups;
    groups.reserve(words.size());
    for (const QString &word : words) {
        QStringList group{word};
        const auto synonym = m_synonyms.constFind(word);
        if (synonym != m_synonyms.cend())
            group.append(*synonym);
        groups.append(std::move(group));
    }
    return groups;
}
