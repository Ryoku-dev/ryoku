#include "busframer.hpp"

#include <QTest>

class TestBusFramer : public QObject
{
    Q_OBJECT

private slots:
    void wholeLines()
    {
        BusFramer f;
        QList<QByteArray> lines;
        f.append("{\"a\":1}\n{\"b\":2}\n", lines);
        QCOMPARE(lines, (QList<QByteArray>{"{\"a\":1}", "{\"b\":2}"}));
        QCOMPARE(f.pending(), 0);
    }

    void splitAcrossReads()
    {
        BusFramer f;
        QList<QByteArray> lines;
        f.append("{\"op\":", lines);
        QVERIFY(lines.isEmpty());
        f.append("\"hello\"", lines);
        QVERIFY(lines.isEmpty());
        f.append("}\n{\"x\"", lines);
        QCOMPARE(lines, QList<QByteArray>{"{\"op\":\"hello\"}"});
        QCOMPARE(f.pending(), 4);
        f.append(":1}\n", lines);
        QCOMPARE(lines.size(), 2);
        QCOMPARE(lines.last(), QByteArray("{\"x\":1}"));
    }

    void byteAtATime()
    {
        const QByteArray text = "{\"a\":\"x\\ny\"}\n\n{\"b\":[1,2]}\n";
        BusFramer f;
        QList<QByteArray> lines;
        for (char c : text)
            f.append(QByteArray(1, c), lines);
        // the escaped \n stays inside its line; the empty line is dropped
        QCOMPARE(lines, (QList<QByteArray>{"{\"a\":\"x\\ny\"}", "{\"b\":[1,2]}"}));
    }

    void largeFrameInChunks()
    {
        QByteArray big = "{\"data\":\"" + QByteArray(200000, 'z') + "\"}";
        BusFramer f;
        QList<QByteArray> lines;
        const QByteArray text = big + '\n';
        for (qsizetype at = 0; at < text.size(); at += 4096)
            f.append(text.mid(at, 4096), lines);
        QCOMPARE(lines.size(), 1);
        QCOMPARE(lines.first(), big);
    }

    void clearDropsPartial()
    {
        BusFramer f;
        QList<QByteArray> lines;
        f.append("{\"half", lines);
        f.clear();
        f.append("{\"a\":1}\n", lines);
        QCOMPARE(lines, QList<QByteArray>{"{\"a\":1}"});
    }
};

QTEST_GUILESS_MAIN(TestBusFramer)
#include "tst_busframer.moc"
