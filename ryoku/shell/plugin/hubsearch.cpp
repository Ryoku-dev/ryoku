#include "hubsearch.hpp"

#include <QJSEngine>
#include <QJSValueIterator>
#include <QQmlEngine>
#include <QRegularExpression>

#include <algorithm>
#include <iterator>
#include <utility>

namespace {

// The full case foldings (CaseFolding.txt status F) that QString::toCaseFolded() leaves
// out, since it applies only the one-to-one mappings: "ß" folds to "ss", so "Straße" and
// "STRASSE" are one word. Generated from Unicode 16.0; every source and result is in the BMP.
struct FullFold {
    char16_t from;
    char16_t to[3];
};

constexpr FullFold fullFolds[] = {
    {0x00DF, {0x0073, 0x0073}}, {0x0130, {0x0069, 0x0307}}, {0x0149, {0x02BC, 0x006E}},
    {0x01F0, {0x006A, 0x030C}}, {0x0390, {0x03B9, 0x0308, 0x0301}}, {0x03B0, {0x03C5, 0x0308, 0x0301}},
    {0x0587, {0x0565, 0x0582}}, {0x1E96, {0x0068, 0x0331}}, {0x1E97, {0x0074, 0x0308}},
    {0x1E98, {0x0077, 0x030A}}, {0x1E99, {0x0079, 0x030A}}, {0x1E9A, {0x0061, 0x02BE}},
    {0x1E9E, {0x0073, 0x0073}}, {0x1F50, {0x03C5, 0x0313}}, {0x1F52, {0x03C5, 0x0313, 0x0300}},
    {0x1F54, {0x03C5, 0x0313, 0x0301}}, {0x1F56, {0x03C5, 0x0313, 0x0342}}, {0x1F80, {0x1F00, 0x03B9}},
    {0x1F81, {0x1F01, 0x03B9}}, {0x1F82, {0x1F02, 0x03B9}}, {0x1F83, {0x1F03, 0x03B9}},
    {0x1F84, {0x1F04, 0x03B9}}, {0x1F85, {0x1F05, 0x03B9}}, {0x1F86, {0x1F06, 0x03B9}},
    {0x1F87, {0x1F07, 0x03B9}}, {0x1F88, {0x1F00, 0x03B9}}, {0x1F89, {0x1F01, 0x03B9}},
    {0x1F8A, {0x1F02, 0x03B9}}, {0x1F8B, {0x1F03, 0x03B9}}, {0x1F8C, {0x1F04, 0x03B9}},
    {0x1F8D, {0x1F05, 0x03B9}}, {0x1F8E, {0x1F06, 0x03B9}}, {0x1F8F, {0x1F07, 0x03B9}},
    {0x1F90, {0x1F20, 0x03B9}}, {0x1F91, {0x1F21, 0x03B9}}, {0x1F92, {0x1F22, 0x03B9}},
    {0x1F93, {0x1F23, 0x03B9}}, {0x1F94, {0x1F24, 0x03B9}}, {0x1F95, {0x1F25, 0x03B9}},
    {0x1F96, {0x1F26, 0x03B9}}, {0x1F97, {0x1F27, 0x03B9}}, {0x1F98, {0x1F20, 0x03B9}},
    {0x1F99, {0x1F21, 0x03B9}}, {0x1F9A, {0x1F22, 0x03B9}}, {0x1F9B, {0x1F23, 0x03B9}},
    {0x1F9C, {0x1F24, 0x03B9}}, {0x1F9D, {0x1F25, 0x03B9}}, {0x1F9E, {0x1F26, 0x03B9}},
    {0x1F9F, {0x1F27, 0x03B9}}, {0x1FA0, {0x1F60, 0x03B9}}, {0x1FA1, {0x1F61, 0x03B9}},
    {0x1FA2, {0x1F62, 0x03B9}}, {0x1FA3, {0x1F63, 0x03B9}}, {0x1FA4, {0x1F64, 0x03B9}},
    {0x1FA5, {0x1F65, 0x03B9}}, {0x1FA6, {0x1F66, 0x03B9}}, {0x1FA7, {0x1F67, 0x03B9}},
    {0x1FA8, {0x1F60, 0x03B9}}, {0x1FA9, {0x1F61, 0x03B9}}, {0x1FAA, {0x1F62, 0x03B9}},
    {0x1FAB, {0x1F63, 0x03B9}}, {0x1FAC, {0x1F64, 0x03B9}}, {0x1FAD, {0x1F65, 0x03B9}},
    {0x1FAE, {0x1F66, 0x03B9}}, {0x1FAF, {0x1F67, 0x03B9}}, {0x1FB2, {0x1F70, 0x03B9}},
    {0x1FB3, {0x03B1, 0x03B9}}, {0x1FB4, {0x03AC, 0x03B9}}, {0x1FB6, {0x03B1, 0x0342}},
    {0x1FB7, {0x03B1, 0x0342, 0x03B9}}, {0x1FBC, {0x03B1, 0x03B9}}, {0x1FC2, {0x1F74, 0x03B9}},
    {0x1FC3, {0x03B7, 0x03B9}}, {0x1FC4, {0x03AE, 0x03B9}}, {0x1FC6, {0x03B7, 0x0342}},
    {0x1FC7, {0x03B7, 0x0342, 0x03B9}}, {0x1FCC, {0x03B7, 0x03B9}}, {0x1FD2, {0x03B9, 0x0308, 0x0300}},
    {0x1FD3, {0x03B9, 0x0308, 0x0301}}, {0x1FD6, {0x03B9, 0x0342}}, {0x1FD7, {0x03B9, 0x0308, 0x0342}},
    {0x1FE2, {0x03C5, 0x0308, 0x0300}}, {0x1FE3, {0x03C5, 0x0308, 0x0301}}, {0x1FE4, {0x03C1, 0x0313}},
    {0x1FE6, {0x03C5, 0x0342}}, {0x1FE7, {0x03C5, 0x0308, 0x0342}}, {0x1FF2, {0x1F7C, 0x03B9}},
    {0x1FF3, {0x03C9, 0x03B9}}, {0x1FF4, {0x03CE, 0x03B9}}, {0x1FF6, {0x03C9, 0x0342}},
    {0x1FF7, {0x03C9, 0x0342, 0x03B9}}, {0x1FFC, {0x03C9, 0x03B9}}, {0xFB00, {0x0066, 0x0066}},
    {0xFB01, {0x0066, 0x0069}}, {0xFB02, {0x0066, 0x006C}}, {0xFB03, {0x0066, 0x0066, 0x0069}},
    {0xFB04, {0x0066, 0x0066, 0x006C}}, {0xFB05, {0x0073, 0x0074}}, {0xFB06, {0x0073, 0x0074}},
    {0xFB13, {0x0574, 0x0576}}, {0xFB14, {0x0574, 0x0565}}, {0xFB15, {0x0574, 0x056B}},
    {0xFB16, {0x057E, 0x0576}}, {0xFB17, {0x0574, 0x056D}}
};

const FullFold *findFullFold(char16_t c)
{
    if (c < 0x00DF)
        return nullptr;
    const auto found = std::lower_bound(std::begin(fullFolds), std::end(fullFolds), c,
                                        [](const FullFold &fold, char16_t value) { return fold.from < value; });
    return found != std::end(fullFolds) && found->from == c ? found : nullptr;
}

// Marks that only accent a letter, dropped so "Schlussel" finds "Schlüssel": the combining
// diacritics NFD splits from Latin, Greek and Cyrillic letters, and the optional Hebrew
// points and Arabic vowel signs. Marks that make a different letter or syllable stay: kana
// voicing marks, Thai and Indic vowel signs.
bool isAccent(char16_t c)
{
    return (c >= 0x0300 && c <= 0x036F) || (c >= 0x1AB0 && c <= 0x1AFF) || (c >= 0x1DC0 && c <= 0x1DFF)
        || (c >= 0x20D0 && c <= 0x20FF) || (c >= 0xFE20 && c <= 0xFE2F)
        || (c >= 0x0591 && c <= 0x05C7 && QChar::isMark(c)) || (c >= 0x0610 && c <= 0x06ED && QChar::isMark(c));
}

bool isAscii(const QString &text)
{
    return std::all_of(text.cbegin(), text.cend(), [](QChar c) { return c.unicode() < 0x80; });
}

// a match without accents counts a little less, so the exact spelling ranks first
constexpr double foldedWeight = 0.99;

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
            const QString label = foldCase(value.property(QStringLiteral("label")).toString());
            const QString description = foldCase(value.property(QStringLiteral("desc")).toString());
            const QString section = foldCase(value.property(QStringLiteral("sectionName")).toString());
            const QString group = foldCase(value.property(QStringLiteral("group")).toString());
            const QString tab = foldCase(value.property(QStringLiteral("tab")).toString());
            const QString keywords = foldCase(value.property(QStringLiteral("kw")).toString());

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
            entry.full = indexWords(full);
            entry.label = indexWords(label);
            entry.keywords = indexWords(keywordText);
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
    static const QRegularExpression whitespace(QStringLiteral("\\s+"), QRegularExpression::UseUnicodePropertiesOption);
    QHash<QString, Group> next;
    if (map.isObject()) {
        QJSValueIterator it(map);
        while (it.hasNext()) {
            it.next();
            Group terms;
            const QStringList words = foldCase(it.value().toString()).split(whitespace, Qt::SkipEmptyParts);
            for (const QString &word : words)
                terms.append(makeTerm(word));
            next.insert(foldCase(it.name()), std::move(terms));
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

    q = foldCase(q.trimmed());
    if (q.isEmpty() || limit <= 0)
        return engine->newArray();

    const QList<Group> groups = expandQuery(q);
    QVector<ScoredEntry> scored;
    scored.reserve(m_entries.size());
    for (qsizetype i = 0; i < m_entries.size(); ++i) {
        const Entry &entry = m_entries.at(i);
        double score = searchScore(groups, entry.full);
        if (score <= 0.0)
            continue;

        score += 2.0 * searchScore(groups, entry.label);
        const double keywordScore = searchScore(groups, entry.keywords);
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

QString HubSearchIndex::foldCase(const QString &text)
{
    const auto special = std::find_if(text.cbegin(), text.cend(), [](QChar c) { return findFullFold(c.unicode()); });
    if (special == text.cend())
        return text.toCaseFolded();

    QString expanded;
    expanded.reserve(text.size() + 8);
    for (const QChar c : text) {
        const FullFold *fold = findFullFold(c.unicode());
        if (!fold) {
            expanded.append(c);
            continue;
        }
        for (const char16_t unit : fold->to) {
            if (unit)
                expanded.append(QChar(unit));
        }
    }
    return expanded.toCaseFolded();
}

QString HubSearchIndex::foldAccents(const QString &word)
{
    if (isAscii(word))
        return word;
    // compatibility forms decompose too: full-width "ｗｉｆｉ" and half-width kana become plain
    const QString decomposed = word.normalized(QString::NormalizationForm_KD);
    QString kept;
    kept.reserve(decomposed.size());
    for (const QChar c : decomposed) {
        if (!isAccent(c.unicode()))
            kept.append(c);
    }
    // what is left composes again, so kana keep their voicing marks and Hangul its syllables
    return foldCase(kept.normalized(QString::NormalizationForm_KC));
}

// scripts written without spaces between words, where one run of letters holds many words
bool HubSearchIndex::isUnspaced(const QString &word)
{
    for (qsizetype i = 0; i < word.size(); ++i) {
        char32_t c = word.at(i).unicode();
        if (word.at(i).isHighSurrogate() && i + 1 < word.size() && word.at(i + 1).isLowSurrogate()) {
            c = QChar::surrogateToUcs4(word.at(i), word.at(i + 1));
            ++i;
        }
        switch (QChar::script(c)) {
        case QChar::Script_Han:
        case QChar::Script_Hiragana:
        case QChar::Script_Katakana:
        case QChar::Script_Thai:
        case QChar::Script_Lao:
        case QChar::Script_Khmer:
        case QChar::Script_Myanmar:
            return true;
        default:
            break;
        }
    }
    return false;
}

QStringList HubSearchIndex::tokenize(const QString &text)
{
    // letters with their combining marks, and digits, make words in every script
    static const QRegularExpression separators(QStringLiteral("[^\\p{L}\\p{M}\\p{N}]+"),
                                               QRegularExpression::UseUnicodePropertiesOption);
    return text.split(separators, Qt::SkipEmptyParts);
}

HubSearchIndex::Words HubSearchIndex::indexWords(const QString &text)
{
    Words out;
    out.words = tokenize(text);
    QStringList folded;
    folded.reserve(out.words.size());
    bool changed = false;
    for (const QString &word : std::as_const(out.words)) {
        QString plain = foldAccents(word);
        changed = changed || plain != word;
        folded.append(std::move(plain));
    }
    if (changed)
        out.folded = std::move(folded);
    return out;
}

HubSearchIndex::Term HubSearchIndex::makeTerm(const QString &word)
{
    Term term{word, foldAccents(word), isUnspaced(word)};
    if (term.folded == term.word)
        term.folded.clear();
    return term;
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

// In a script written without spaces a word sits inside a longer run of letters, where
// indexOf finds it as it finds any substring. Such words skip the scattered-letter
// fallback: a long Japanese or Thai run holds most short words' letters in some order.
double HubSearchIndex::wordScore(const QString &word, const QStringList &textWords, bool unspaced)
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
        if (!unspaced && word.size() <= textWord.size()) {
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

double HubSearchIndex::termScore(const Term &term, const Words &text)
{
    const double score = wordScore(term.word, text.words, term.unspaced);
    if (term.folded.isEmpty() && text.folded.isEmpty())
        return score;
    // either side without accents; a side with none to drop is already its own folded form
    const QString &word = term.folded.isEmpty() ? term.word : term.folded;
    const QStringList &words = text.folded.isEmpty() ? text.words : text.folded;
    return std::max(score, foldedWeight * wordScore(word, words, term.unspaced));
}

double HubSearchIndex::searchScore(const QList<Group> &groups, const Words &text)
{
    double total = 0.0;
    int matched = 0;
    for (const Group &group : groups) {
        double best = 0.0;
        for (qsizetype i = 0; i < group.size(); ++i) {
            double score = termScore(group.at(i), text);
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

// the query splits into words the way entries do, so "キーボード・ショートカット" is two words
QList<HubSearchIndex::Group> HubSearchIndex::expandQuery(const QString &query) const
{
    const QStringList words = tokenize(query);
    QList<Group> groups;
    groups.reserve(words.size());
    for (const QString &word : words) {
        Group group{makeTerm(word)};
        auto synonym = m_synonyms.constFind(word);
        if (synonym == m_synonyms.cend() && !group.first().folded.isEmpty())
            synonym = m_synonyms.constFind(group.first().folded);
        if (synonym != m_synonyms.cend())
            group.append(*synonym);
        groups.append(std::move(group));
    }
    return groups;
}
