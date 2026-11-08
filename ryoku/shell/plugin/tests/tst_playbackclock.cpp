#include "playbackclock.hpp"

#include <QElapsedTimer>
#include <QSignalSpy>
#include <QTest>

class TestPlaybackClock : public QObject {
    Q_OBJECT

private slots:
    void advancesOnlyWhileRunning() {
        PlaybackClock clock;
        clock.setInterval(5);
        clock.reanchor(1.0);

        QSignalSpy elapsedChanged(&clock, &PlaybackClock::elapsedChanged);
        clock.setRunning(true);
        QTRY_VERIFY_WITH_TIMEOUT(clock.elapsed() >= 1.02, 250);
        QVERIFY(!elapsedChanged.isEmpty());

        clock.setRunning(false);
        const double stopped = clock.elapsed();
        const qsizetype changes = elapsedChanged.size();
        QTest::qWait(30);
        QCOMPARE(clock.elapsed(), stopped);
        QCOMPARE(elapsedChanged.size(), changes);
    }

    void reanchorUpdatesImmediately() {
        PlaybackClock clock;
        clock.reanchor(12.5);
        QCOMPARE(clock.anchorPosition(), 12.5);
        QCOMPARE(clock.elapsed(), 12.5);

        clock.reanchor(3.25);
        QCOMPARE(clock.anchorPosition(), 3.25);
        QCOMPARE(clock.elapsed(), 3.25);
    }

    void rateControlsAdvance() {
        PlaybackClock clock;
        clock.setInterval(5);
        clock.setRate(2.0);
        clock.reanchor(0.0);

        QElapsedTimer wall;
        wall.start();
        clock.setRunning(true);
        QTest::qWait(50);
        clock.setRunning(false);

        const double wallSeconds = static_cast<double>(wall.elapsed()) / 1000.0;
        QVERIFY(clock.elapsed() >= wallSeconds * 1.6);
        QVERIFY(clock.elapsed() <= wallSeconds * 2.4);
    }

    void lengthClampsPositions() {
        PlaybackClock clock;
        clock.setLength(-4.0);
        QCOMPARE(clock.length(), 0.0);

        clock.setLength(10.0);
        clock.reanchor(14.0);
        QCOMPARE(clock.anchorPosition(), 10.0);
        QCOMPARE(clock.elapsed(), 10.0);

        clock.reanchor(-2.0);
        QCOMPARE(clock.anchorPosition(), 0.0);
        QCOMPARE(clock.elapsed(), 0.0);
    }

    void lineIndexEdges() {
        PlaybackClock clock;
        QCOMPARE(clock.lineIndex(), -1);

        clock.setLineTimes({1.0, 3.0, 5.0});
        clock.reanchor(0.999);
        QCOMPARE(clock.lineIndex(), -1);

        clock.reanchor(1.0);
        QCOMPARE(clock.lineIndex(), 0);

        clock.reanchor(3.0);
        QCOMPARE(clock.lineIndex(), 1);

        clock.reanchor(50.0);
        QCOMPARE(clock.lineIndex(), 2);

        clock.setLineTimes({});
        QCOMPARE(clock.lineIndex(), -1);
    }
};

QTEST_GUILESS_MAIN(TestPlaybackClock)
#include "tst_playbackclock.moc"
