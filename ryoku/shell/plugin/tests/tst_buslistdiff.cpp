#include "buslistdiff.hpp"

#include <QRandomGenerator>
#include <QTest>

#include <algorithm>
#include <random>

using buslistdiff::Op;

namespace {

QStringList keys(const char *spaced)
{
    return QString::fromLatin1(spaced).split(QLatin1Char(' '), Qt::SkipEmptyParts);
}

int countOf(const QList<Op> &ops, Op::Kind kind)
{
    int n = 0;
    for (const Op &op : ops)
        if (op.kind == kind)
            n += kind == Op::Move ? 1 : op.count;
    return n;
}

}

class TestBusListDiff : public QObject
{
    Q_OBJECT

private slots:
    void same()
    {
        QVERIFY(buslistdiff::diff(keys("a b c"), keys("a b c")).isEmpty());
    }

    void cases_data()
    {
        QTest::addColumn<QStringList>("before");
        QTest::addColumn<QStringList>("after");
        QTest::addColumn<int>("removes");
        QTest::addColumn<int>("inserts");
        QTest::addColumn<int>("moves");
        QTest::newRow("insert middle") << keys("a b c") << keys("a x b c") << 0 << 1 << 0;
        QTest::newRow("insert run") << keys("a b") << keys("x y a b z") << 0 << 3 << 0;
        QTest::newRow("remove middle") << keys("a b c d") << keys("a d") << 2 << 0 << 0;
        QTest::newRow("remove all") << keys("a b") << QStringList() << 2 << 0 << 0;
        QTest::newRow("from empty") << QStringList() << keys("a b") << 0 << 2 << 0;
        QTest::newRow("last to front") << keys("a b c d e") << keys("e a b c d") << 0 << 0 << 1;
        QTest::newRow("front to back") << keys("a b c d e") << keys("b c d e a") << 0 << 0 << 1;
        QTest::newRow("swap") << keys("a b c d") << keys("a c b d") << 0 << 0 << 1;
        QTest::newRow("mixed") << keys("a b c d e f") << keys("f x a c e") << 2 << 1 << 1;
        QTest::newRow("reverse") << keys("a b c d") << keys("d c b a") << 0 << 0 << -1;
    }

    void cases()
    {
        QFETCH(QStringList, before);
        QFETCH(QStringList, after);
        QFETCH(int, removes);
        QFETCH(int, inserts);
        QFETCH(int, moves);
        const QList<Op> ops = buslistdiff::diff(before, after);
        QCOMPARE(buslistdiff::replay(before, after, ops), after);
        QCOMPARE(countOf(ops, Op::Remove), removes);
        QCOMPARE(countOf(ops, Op::Insert), inserts);
        if (moves >= 0)
            QCOMPARE(countOf(ops, Op::Move), moves);
    }

    // every op must be valid at the moment it runs, like a model emits them
    void randomShuffles()
    {
        QRandomGenerator rng(7);
        for (int round = 0; round < 300; ++round) {
            QStringList before;
            const int n = rng.bounded(40);
            for (int i = 0; i < n; ++i)
                before.append(QStringLiteral("k%1").arg(i));
            QStringList after = before;
            std::shuffle(after.begin(), after.end(), std::mt19937(rng.generate()));
            after = after.mid(0, rng.bounded(after.size() + 1));
            for (int i = rng.bounded(6); i > 0; --i)
                after.insert(rng.bounded(after.size() + 1), QStringLiteral("n%1_%2").arg(round).arg(i));
            const QList<Op> ops = buslistdiff::diff(before, after);
            QStringList cur = before;
            for (const Op &op : ops) {
                switch (op.kind) {
                case Op::Remove:
                    QVERIFY(op.row >= 0 && op.row + op.count <= cur.size());
                    break;
                case Op::Insert:
                    QVERIFY(op.row >= 0 && op.row <= cur.size());
                    break;
                case Op::Move:
                    QVERIFY(op.row >= 0 && op.row < cur.size() && op.to >= 0 && op.to < cur.size());
                    QVERIFY(op.row != op.to);
                    break;
                }
                cur = buslistdiff::replay(cur, after, {op});
            }
            QCOMPARE(cur, after);
        }
    }
};

QTEST_GUILESS_MAIN(TestBusListDiff)
#include "tst_buslistdiff.moc"
