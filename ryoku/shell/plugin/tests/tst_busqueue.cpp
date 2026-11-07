#include "busworker.hpp"

#include <QJsonDocument>
#include <QTest>

namespace {

BusInbound full(const char *topic, qint64 rev, const char *data)
{
    BusInbound in;
    in.kind = BusInbound::State;
    in.topic = QString::fromLatin1(topic);
    in.rev = rev;
    in.data = QJsonDocument::fromJson(data).object();
    return in;
}

BusInbound delta(const char *topic, qint64 rev, const char *data, const char *patch)
{
    BusInbound in = full(topic, rev, data);
    in.delta = true;
    in.patch = QJsonDocument::fromJson(patch).array();
    return in;
}

BusInbound other(BusInbound::Kind kind, const char *topic = "")
{
    BusInbound in;
    in.kind = kind;
    in.topic = QString::fromLatin1(topic);
    return in;
}

}

// The hand-off queue between the worker and the GUI thread.
class TestBusQueue : public QObject
{
    Q_OBJECT

private slots:
    void newestFullFrameWins()
    {
        QList<BusInbound> q;
        BusWorker::enqueue(q, full("wm", 1, R"({"a":1})"));
        BusWorker::enqueue(q, other(BusInbound::Reply));
        BusWorker::enqueue(q, full("wm", 2, R"({"a":2})"));
        QCOMPARE(q.size(), 2);
        QCOMPARE(q.at(0).kind, BusInbound::Reply);
        QCOMPARE(q.at(1).rev, 2);
        QCOMPARE(q.at(1).data.toObject().value("a").toInt(), 2);
    }

    void deltasChain()
    {
        QList<BusInbound> q;
        BusWorker::enqueue(q, delta("settings", 2, R"({"a":1,"b":1})", R"([{"p":["a"],"v":1}])"));
        BusWorker::enqueue(q, delta("settings", 3, R"({"a":1,"b":2})", R"([{"p":["b"],"v":2}])"));
        QCOMPARE(q.size(), 1);
        QVERIFY(q.first().delta);
        QCOMPARE(q.first().rev, 3);
        QCOMPARE(q.first().patch.size(), 2);
        QCOMPARE(q.first().data.toObject().value("b").toInt(), 2);
    }

    void deltaOnFullStaysFull()
    {
        QList<BusInbound> q;
        BusWorker::enqueue(q, full("settings", 1, R"({"a":0})"));
        BusWorker::enqueue(q, delta("settings", 2, R"({"a":1})", R"([{"p":["a"],"v":1}])"));
        QCOMPARE(q.size(), 1);
        QVERIFY(!q.first().delta);
        QCOMPARE(q.first().rev, 2);
        QCOMPARE(q.first().data.toObject().value("a").toInt(), 1);
        // and a full frame after a delta replaces it
        BusWorker::enqueue(q, delta("settings", 3, R"({"a":2})", R"([{"p":["a"],"v":2}])"));
        BusWorker::enqueue(q, full("settings", 4, R"({"z":1})"));
        QCOMPARE(q.size(), 1);
        QVERIFY(!q.first().delta);
        QVERIFY(q.first().patch.isEmpty());
    }

    void eventsAndOtherTopicsKept()
    {
        QList<BusInbound> q;
        BusWorker::enqueue(q, other(BusInbound::Event, "keypress"));
        BusWorker::enqueue(q, other(BusInbound::Event, "keypress"));
        BusWorker::enqueue(q, full("wm", 1, "{}"));
        BusWorker::enqueue(q, full("net", 1, "{}"));
        QCOMPARE(q.size(), 4);
    }

    void neverAcrossAReconnect()
    {
        QList<BusInbound> q;
        BusWorker::enqueue(q, full("wm", 5, "{}"));
        BusWorker::enqueue(q, other(BusInbound::Closed));
        BusWorker::enqueue(q, other(BusInbound::Opened));
        BusWorker::enqueue(q, full("wm", 1, "{}"));
        QCOMPARE(q.size(), 4);
        QCOMPARE(q.first().rev, 5);
    }
};

QTEST_GUILESS_MAIN(TestBusQueue)
#include "tst_busqueue.moc"
