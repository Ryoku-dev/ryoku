#include "fuzzyranker.hpp"

#include <QJSEngine>
#include <QtTest>

class TestFuzzyRanker : public QObject
{
    Q_OBJECT

    QJSEngine engine;
    FuzzyRanker ranker{&engine};

    QJSValue apps()
    {
        return engine.evaluate(R"([
            { id: "spotify.desktop", name: "Spotify", genericName: "Musik", keywords: ["audio", "stream"], noDisplay: false },
            { id: "files.desktop", name: "Files", genericName: "File Manager", keywords: [], noDisplay: false },
            { id: "firefox.desktop", name: "Firefox", genericName: "Web Browser", keywords: ["www", "internet"], noDisplay: false },
            { id: "settings-daemon.desktop", name: "Settings Daemon", genericName: "", keywords: [], noDisplay: true },
            { id: "steam.desktop", name: "Steam", genericName: "Game Launcher", keywords: ["games"], noDisplay: false }
        ])");
    }

    QStringList names(const QString &query, const QJSValue &usage = QJSValue())
    {
        const QJSValue result = ranker.rank(apps(), query, usage);
        QStringList out;
        const quint32 length = result.property(QStringLiteral("length")).toUInt();
        for (quint32 i = 0; i < length; ++i)
            out.append(result.property(i).property(QStringLiteral("name")).toString());
        return out;
    }

    static QStringList namesOf(const QJSValue &result, const QString &field = QStringLiteral("name"))
    {
        QStringList out;
        const quint32 length = result.property(QStringLiteral("length")).toUInt();
        for (quint32 i = 0; i < length; ++i)
            out.append(result.property(i).property(field).toString());
        return out;
    }

    static QList<QPair<int, int>> spansOf(const QJSValue &result)
    {
        QList<QPair<int, int>> out;
        const quint32 length = result.property(QStringLiteral("length")).toUInt();
        for (quint32 i = 0; i < length; ++i) {
            const QJSValue span = result.property(i);
            out.append({span.property(QStringLiteral("start")).toInt(),
                        span.property(QStringLiteral("len")).toInt()});
        }
        return out;
    }

private slots:
    void subsequenceScoring()
    {
        const QJSValue spotify = apps().property(0);
        QCOMPARE(ranker.score(spotify, QStringLiteral("spt")), 2);
        QCOMPARE(ranker.score(spotify, QStringLiteral("xyz")), 99);
    }

    void emptyQueryListsVisibleAppsAlphabetically()
    {
        QCOMPARE(names(QString()),
                 QStringList({QStringLiteral("Files"), QStringLiteral("Firefox"),
                              QStringLiteral("Spotify"), QStringLiteral("Steam")}));
    }

    void prefixMatchesRankFirst()
    {
        QCOMPARE(names(QStringLiteral("spo")).first(), QStringLiteral("Spotify"));
        QCOMPARE(names(QStringLiteral("fi")),
                 QStringList({QStringLiteral("Files"), QStringLiteral("Firefox")}));
    }

    void genericNameAndKeywordMatch()
    {
        QVERIFY(names(QStringLiteral("musik")).contains(QStringLiteral("Spotify")));
        QVERIFY(names(QStringLiteral("internet")).contains(QStringLiteral("Firefox")));
        QCOMPARE(names(QStringLiteral("gam")), QStringList({QStringLiteral("Steam")}));
    }

    void noMatchReturnsEmpty()
    {
        QVERIFY(names(QStringLiteral("zzz")).isEmpty());
    }

    void noDisplayIsAlwaysExcluded()
    {
        const QJSValue source = apps();
        const QJSValue result = ranker.rank(source, QStringLiteral("s"));
        const QJSValue hidden = source.property(3);
        const quint32 length = result.property(QStringLiteral("length")).toUInt();
        for (quint32 i = 0; i < length; ++i)
            QVERIFY(!result.property(i).strictlyEquals(hidden));
    }

    void rankIsCaseInsensitive()
    {
        QCOMPARE(names(QStringLiteral("Spo")), QStringList({QStringLiteral("Spotify")}));
    }

    void usageOrdersEmptyQuery()
    {
        const QJSValue usage = engine.evaluate(R"(({ "steam.desktop": 5, "firefox.desktop": 2 }))");
        QCOMPARE(names(QString(), usage),
                 QStringList({QStringLiteral("Steam"), QStringLiteral("Firefox"),
                              QStringLiteral("Files"), QStringLiteral("Spotify")}));
        QCOMPARE(names(QString()),
                 QStringList({QStringLiteral("Files"), QStringLiteral("Firefox"),
                              QStringLiteral("Spotify"), QStringLiteral("Steam")}));
    }

    void usageBreaksQueryTies()
    {
        const QJSValue usage = engine.evaluate(R"(({ "firefox.desktop": 3 }))");
        QCOMPARE(names(QStringLiteral("fi"), usage),
                 QStringList({QStringLiteral("Firefox"), QStringLiteral("Files")}));
        QCOMPARE(names(QStringLiteral("fi")),
                 QStringList({QStringLiteral("Files"), QStringLiteral("Firefox")}));
    }

    void rankReturnsOriginalObjects()
    {
        const QJSValue source = apps();
        const QJSValue result = ranker.rank(source, QStringLiteral("spo"));
        QVERIFY(result.property(0).strictlyEquals(source.property(0)));
    }

    void stableTiesAndCommonLatinCaseFolding()
    {
        const QJSValue source = engine.evaluate(R"([
            { id: "first", name: "Éclair" },
            { id: "second", name: "Éclair" }
        ])");
        const QJSValue result = ranker.rank(source, QStringLiteral("ÉC"));
        QCOMPARE(result.property(QStringLiteral("length")).toInt(), 2);
        QVERIFY(result.property(0).strictlyEquals(source.property(0)));
        QVERIFY(result.property(1).strictlyEquals(source.property(1)));
    }

    void missingFieldsStaySafe()
    {
        const QJSValue source = engine.evaluate(R"([
            { id: "empty" },
            { id: "named", name: "Beta", genericName: null, keywords: null }
        ])");
        const QJSValue result = ranker.rank(source, QStringLiteral("bet"));
        QCOMPARE(result.property(QStringLiteral("length")).toInt(), 1);
        QVERIFY(result.property(0).strictlyEquals(source.property(1)));
        QCOMPARE(ranker.score(source.property(0), QStringLiteral("x")), 99);
    }

    void highlightSpans()
    {
        const QList<QPair<int, int>> prefix{{0, 3}};
        const QList<QPair<int, int>> scattered{{0, 1}, {3, 1}, {6, 1}};
        const QList<QPair<int, int>> merged{{0, 2}, {4, 1}};
        QCOMPARE(spansOf(ranker.highlight(QStringLiteral("Firefox"), QStringLiteral("fir"))), prefix);
        QCOMPARE(spansOf(ranker.highlight(QStringLiteral("Spotify"), QStringLiteral("sty"))), scattered);
        QCOMPARE(spansOf(ranker.highlight(QStringLiteral("Files"), QStringLiteral("fis"))), merged);
        QCOMPARE(spansOf(ranker.highlight(QStringLiteral("Firefox"), QStringLiteral("FIR"))), prefix);
        QVERIFY(spansOf(ranker.highlight(QStringLiteral("Firefox"), QStringLiteral("zzz"))).isEmpty());
        QVERIFY(spansOf(ranker.highlight(QStringLiteral("Firefox"), QString())).isEmpty());
    }

    void prefixRankOrdersPrefixThenContains()
    {
        const QJSValue source = engine.evaluate(R"([
            { name: "My Alpha" },
            { name: "Alpha" },
            { name: "Almanac" },
            { name: "beta alpha" },
            { name: "Zulu" }
        ])");
        QCOMPARE(namesOf(ranker.prefixRank(source, QStringLiteral("al"), QStringLiteral("name"))),
                 QStringList({QStringLiteral("Almanac"), QStringLiteral("Alpha"),
                              QStringLiteral("beta alpha"), QStringLiteral("My Alpha")}));
        QCOMPARE(namesOf(ranker.prefixRank(source, QStringLiteral("ha"), QStringLiteral("name"))),
                 QStringList({QStringLiteral("Alpha"), QStringLiteral("beta alpha"),
                              QStringLiteral("My Alpha")}));
    }

    void prefixRankSortsIdleRowsAndReturnsOriginalObjects()
    {
        const QJSValue source = engine.evaluate(R"([
            { title: "Zulu", noDisplay: true },
            { title: "Alpha" }
        ])");
        const QJSValue result = ranker.prefixRank(source, QStringLiteral("  "), QStringLiteral("title"));
        QCOMPARE(namesOf(result, QStringLiteral("title")),
                 QStringList({QStringLiteral("Alpha"), QStringLiteral("Zulu")}));
        QVERIFY(result.property(1).strictlyEquals(source.property(0)));
    }
};

QTEST_GUILESS_MAIN(TestFuzzyRanker)
#include "tst_fuzzyranker.moc"
