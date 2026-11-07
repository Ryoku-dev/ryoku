#include "bustestenv.hpp"
#include "busworker.hpp"
#include "fakedaemon.hpp"

#include <QJsonArray>
#include <QJsonDocument>

namespace {

QJsonObject obj(const char *text)
{
    return QJsonDocument::fromJson(text).object();
}

}

class TestBusClient : public QObject
{
    Q_OBJECT

    // A JS collector: rec(...) appends its arguments to got.
    QJSValue collector(QQmlEngine &engine)
    {
        engine.globalObject().setProperty("got", engine.newArray());
        return engine.evaluate("(function () { got.push(Array.prototype.slice.call(arguments)); })");
    }

    QJSValue got(QQmlEngine &engine) { return engine.globalObject().property("got"); }
    int gotCount(QQmlEngine &engine) { return got(engine).property("length").toInt(); }
    QJsonValue gotAt(QQmlEngine &engine, int i, int arg)
    {
        return engine.fromScriptValue<QJsonValue>(got(engine).property(quint32(i)).property(quint32(arg)));
    }

private slots:
    void backoff()
    {
        QCOMPARE(BusWorker::backoff(0), 250);
        QCOMPARE(BusWorker::backoff(1), 500);
        QCOMPARE(BusWorker::backoff(4), 4000);
        QCOMPARE(BusWorker::backoff(5), 5000);
        QCOMPARE(BusWorker::backoff(400), 5000);
    }

    void handshakeAndFullFrame()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        QVERIFY(bus);
        QCOMPARE(bus->clientName(), QStringLiteral("test"));
        QCOMPARE(bus->socketPath(), env.dir.path() + "/ryoku-shell.sock");

        bus->subscribe("wm", collector(engine));
        QTRY_COMPARE(daemon.ops("sub").size(), 1);
        const QJsonObject hello = daemon.ops("bus").first();
        QCOMPARE(hello.value("v").toInt(), 1);
        QCOMPARE(hello.value("client").toString(), QStringLiteral("test"));
        QVERIFY(hello.value("pid").toInteger() > 0);
        QVERIFY(daemon.lines().first().startsWith("bus "));
        QCOMPARE(daemon.ops("sub").first().value("topics").toArray(), QJsonArray{"wm"});
        QVERIFY(!bus->connected());

        daemon.send(obj(R"({"op":"hello","v":1,"topics":["wm"]})"));
        QTRY_VERIFY(bus->connected());
        daemon.send(obj(R"({"op":"state","topic":"wm","rev":3,"data":{"windows":[{"id":1}]}})"));
        QTRY_COMPARE(gotCount(engine), 1);
        QCOMPARE(gotAt(engine, 0, 0), QJsonValue(obj(R"({"windows":[{"id":1}]})")));
        QCOMPARE(gotAt(engine, 0, 1), QJsonValue(obj(R"({"rev":3,"paths":null})")));

        // a late subscriber gets the cached frame at once
        const QJSValue late = collector(engine);
        bus->subscribe("wm", late);
        QCOMPARE(gotCount(engine), 1);
        // a frame for a topic nobody reads is dropped
        daemon.send(obj(R"({"op":"state","topic":"other","rev":1,"data":{}})"));
        daemon.send(obj(R"({"op":"event","topic":"other","seq":1,"data":{}})"));
        QTest::qWait(50);
        QCOMPARE(gotCount(engine), 1);
    }

    void patchBaseRevAndResync()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        bus->subscribe("settings", collector(engine));
        QTRY_COMPARE(daemon.ops("sub").size(), 1);
        daemon.send(obj(R"({"op":"hello","v":1})"));
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":1,"data":{"bar":{"style":"o"},"dock":{"n":1}}})"));
        QTRY_COMPARE(gotCount(engine), 1);

        daemon.send(obj(R"({"op":"state","topic":"settings","rev":2,"base":1,"patch":[{"p":["bar","style"],"v":"k"}]})"));
        QTRY_COMPARE(gotCount(engine), 2);
        QCOMPARE(gotAt(engine, 1, 0), QJsonValue(obj(R"({"bar":{"style":"k"},"dock":{"n":1}})")));
        QCOMPARE(gotAt(engine, 1, 1), QJsonValue(obj(R"({"rev":2,"paths":[["bar","style"]]})")));
        // untouched subtrees keep their identity across the patch
        QJSValue first = got(engine).property(0).property(0);
        QJSValue second = got(engine).property(1).property(0);
        QVERIFY(first.property("dock").strictlyEquals(second.property("dock")));

        // a gap: the client asks for the topic again, once, and skips the patch
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":5,"base":4,"patch":[{"p":["x"],"v":1}]})"));
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":6,"base":5,"patch":[{"p":["y"],"v":1}]})"));
        QTRY_COMPARE(daemon.ops("sub").size(), 2);
        QTest::qWait(50);
        QCOMPARE(daemon.ops("sub").size(), 2);
        QCOMPARE(gotCount(engine), 2);
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":6,"data":{"bar":{"style":"r"}}})"));
        QTRY_COMPARE(gotCount(engine), 3);
        QCOMPARE(gotAt(engine, 2, 1), QJsonValue(obj(R"({"rev":6,"paths":null})")));
        // back in step: patches apply again
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":7,"base":6,"patch":[{"p":["bar"],"x":true}]})"));
        QTRY_COMPARE(gotCount(engine), 4);
        QCOMPARE(gotAt(engine, 3, 0), QJsonValue(QJsonObject()));
    }

    void requestsAndReplies()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        const QJSValue cb = collector(engine);
        const int a = bus->call("settings.patch", engine.toScriptValue(obj(R"({"path":"a","value":1})")), cb);
        const int b = bus->run("ping", cb);
        const int c = bus->ipc("shell", "open", engine.evaluate("[1, true, 'x']"), cb, QJSValue("hub"));
        const int d = bus->ipcGet("shell", "ready", cb);
        QVERIFY(a < b && b < c && c < d);
        QTRY_COMPARE(daemon.ops("ipc").size(), 2);
        QCOMPARE(daemon.ops("call").first(), obj(QStringLiteral(R"({"op":"call","id":%1,"method":"settings.patch","args":{"path":"a","value":1}})").arg(a).toUtf8()));
        QCOMPARE(daemon.ops("run").first(), obj(QStringLiteral(R"({"op":"run","id":%1,"line":"ping"})").arg(b).toUtf8()));
        QCOMPARE(daemon.ops("ipc").first().value("args").toArray(), (QJsonArray{"1", "true", "x"}));
        QCOMPARE(daemon.ops("ipc").first().value("client").toString(), QStringLiteral("hub"));
        QCOMPARE(daemon.ops("ipc").last().value("prop").toString(), QStringLiteral("ready"));
        QVERIFY(!daemon.ops("ipc").last().contains("client"));

        daemon.send(QJsonObject{{"op", "reply"}, {"id", a}, {"ok", true}, {"result", QJsonObject{{"n", 2}}}});
        daemon.send(QJsonObject{{"op", "reply"}, {"id", b}, {"ok", false}, {"error", "nope"}});
        daemon.send(QJsonObject{{"op", "reply"}, {"id", c}, {"ok", true}});
        QTRY_COMPARE(gotCount(engine), 3);
        QCOMPARE(gotAt(engine, 0, 0), QJsonValue(true));
        QCOMPARE(gotAt(engine, 0, 1), QJsonValue(obj(R"({"n":2})")));
        QCOMPARE(gotAt(engine, 0, 2), QJsonValue(""));
        QCOMPARE(gotAt(engine, 1, 0), QJsonValue(false));
        QCOMPARE(gotAt(engine, 1, 1), QJsonValue(""));
        QCOMPARE(gotAt(engine, 1, 2), QJsonValue("nope"));
        // run and ipc turn a missing result into ""
        QCOMPARE(gotAt(engine, 2, 1), QJsonValue(""));

        // the one still in flight fails when the connection drops
        daemon.drop();
        QTRY_COMPARE(gotCount(engine), 4);
        QCOMPARE(gotAt(engine, 3, 2), QJsonValue("disconnected"));
    }

    void queueWhileDownAndCancel()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        const QJSValue cb = collector(engine);
        const int first = bus->run("one", cb);
        const int second = bus->run("two", cb);
        QTest::qWait(100);
        QCOMPARE(bus->queued(), 2);
        QVERIFY(bus->cancel(first));
        QVERIFY(!bus->cancel(first));
        for (int i = 0; i < BusClient::QueueMax; ++i)
            bus->run(QStringLiteral("fill%1").arg(i), cb);
        // the oldest is dropped with an error once the queue is full
        QCOMPARE(bus->queued(), BusClient::QueueMax);
        QCOMPARE(gotCount(engine), 1);
        QCOMPARE(gotAt(engine, 0, 2), QJsonValue("queue full"));
        QVERIFY(!bus->cancel(second));

        QVERIFY(daemon.listen());
        QTRY_COMPARE(daemon.ops("run").size(), BusClient::QueueMax);
        QVERIFY(daemon.lines().first().startsWith("bus "));
        QCOMPARE(daemon.ops("run").first().value("line").toString(), QStringLiteral("fill0"));
        QCOMPARE(bus->queued(), 0);
    }

    void reconnectResubscribesAndReserves()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        bus->subscribe("wm", collector(engine));
        const QJSValue handler = engine.evaluate("({ ping: function () { return 'pong'; } })");
        bus->serve("shell", handler);
        QTRY_COMPARE(daemon.ops("serve").size(), 1);
        daemon.send(obj(R"({"op":"hello","v":1})"));
        daemon.send(obj(R"({"op":"state","topic":"wm","rev":1,"data":{"a":1}})"));
        QTRY_COMPARE(gotCount(engine), 1);

        daemon.drop();
        QTRY_VERIFY(!bus->connected());
        QTRY_COMPARE(daemon.connections(), 2);
        QTRY_COMPARE(daemon.ops("serve").size(), 1);
        QCOMPARE(daemon.ops("sub").first().value("topics").toArray(), QJsonArray{"wm"});
        QCOMPARE(daemon.ops("serve").first().value("targets").toArray(), QJsonArray{"shell"});
        daemon.send(obj(R"({"op":"hello","v":1})"));
        QTRY_VERIFY(bus->connected());
        // the resync frame replaces the cached one
        daemon.send(obj(R"({"op":"state","topic":"wm","rev":9,"data":{"a":2}})"));
        QTRY_COMPARE(gotCount(engine), 2);
        QCOMPARE(gotAt(engine, 1, 0), QJsonValue(obj(R"({"a":2})")));

        bus->unserve("shell", handler);
        QTRY_COMPARE(daemon.ops("unserve").size(), 1);
    }

    void cmdDispatch()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        const QJSValue older = engine.evaluate("({ greet: function (n) { return 'old ' + n; } })");
        const QJSValue handler = engine.evaluate(R"(({
            greet: function (n) { return 'hi ' + n; },
            kind: function (v) { return typeof v + ':' + v; },
            nothing: function () {},
            boom: function () { throw new Error('bad'); },
            _hidden: function () { return 1; },
            sizeChanged: function () { return 1; },
            size: 7
        }))");
        bus->serve("shell", older);
        bus->serve("shell", handler);
        bus->serve("shell", handler);
        QTRY_COMPARE(daemon.ops("serve").size(), 1);
        const auto cmd = [&](int id, const char *rest) {
            daemon.sendRaw(QStringLiteral("{\"op\":\"cmd\",\"id\":%1,%2}\n").arg(id).arg(QLatin1String(rest)).toUtf8());
        };
        cmd(1, R"("target":"shell","fn":"greet","args":["ann"])");
        cmd(2, R"("target":"shell","fn":"kind","args":["true"])");
        cmd(3, R"("target":"shell","fn":"kind","args":["12"])");
        cmd(4, R"("target":"shell","fn":"greet","args":[])");
        cmd(5, R"("target":"shell","fn":"missing","args":[])");
        cmd(6, R"("target":"nobody","fn":"greet","args":["x"])");
        cmd(7, R"("target":"shell","prop":"size")");
        cmd(8, R"("target":"shell","fn":"_hidden","args":[])");
        cmd(9, R"("target":"shell","fn":"sizeChanged","args":[])");
        cmd(10, R"("target":"shell","fn":"nothing","args":[])");
        cmd(11, R"("target":"shell","fn":"boom","args":[])");
        cmd(12, R"("target":"shell","prop":"greet")");
        QTRY_COMPARE(daemon.ops("done").size(), 12);
        const QList<QJsonObject> done = daemon.ops("done");
        const auto expect = [&](int i, bool ok, const QString &text) {
            const QJsonObject d = done.at(i);
            QCOMPARE(d.value("id").toInt(), i + 1);
            QCOMPARE(d.value("ok").toBool(), ok);
            QCOMPARE(d.value(ok ? "result" : "error").toString(), text);
        };
        expect(0, true, "hi ann");
        expect(1, true, "boolean:true");
        expect(2, true, "string:12");
        expect(3, false, "Too few arguments provided (1 required but 0 were provided.)");
        expect(4, false, "function not found");
        expect(5, false, "target not found");
        expect(6, true, "7");
        expect(7, false, "function not found");
        expect(8, false, "function not found");
        expect(9, true, "");
        expect(10, false, "Error: bad");
        expect(11, false, "property not found");

        // the newest handler wins; when it leaves the older one answers again
        bus->serveAs(handler, "");
        cmd(13, R"("target":"shell","fn":"greet","args":["bo"])");
        QTRY_COMPARE(daemon.ops("done").size(), 13);
        QCOMPARE(daemon.ops("done").last().value("result").toString(), QStringLiteral("old bo"));
        QCOMPARE(daemon.ops("unserve").size(), 0);
        bus->serveAs(older, "other");
        QTRY_COMPARE(daemon.ops("unserve").size(), 1);
        QTRY_COMPARE(daemon.ops("serve").size(), 2);
    }

    void eventsAndSeed()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        BusClient *bus = BusTestEnv::client(engine);
        const QJSValue cb = collector(engine);
        bus->subscribe("keypress", cb);
        bus->subscribe("settings", cb);
        // before the daemon has sent settings, a seed lands for every reader
        bus->seed("settings", engine.toScriptValue(obj(R"({"language":"de"})")));
        QCOMPARE(gotCount(engine), 1);
        QCOMPARE(gotAt(engine, 0, 1), QJsonValue(obj(R"({"rev":0,"paths":null})")));
        // one sub line in the handshake or one per topic, by timing
        QTRY_VERIFY([&] {
            int n = 0;
            for (const QJsonObject &s : daemon.ops("sub"))
                n += int(s.value("topics").toArray().size());
            return n == 2;
        }());
        daemon.send(obj(R"({"op":"hello","v":1})"));
        daemon.send(obj(R"({"op":"event","topic":"keypress","seq":4,"dropped":2,"data":{"keys":["a"]}})"));
        QTRY_COMPARE(gotCount(engine), 2);
        QCOMPARE(gotAt(engine, 1, 0), QJsonValue(obj(R"({"keys":["a"]})")));
        QCOMPARE(gotAt(engine, 1, 1), QJsonValue(obj(R"({"seq":4,"dropped":2,"event":true})")));
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":2,"data":{"language":"en"}})"));
        QTRY_COMPARE(gotCount(engine), 3);
        // the daemon's frame now wins over a seed
        bus->seed("settings", engine.toScriptValue(obj(R"({"language":"fr"})")));
        QCOMPARE(gotCount(engine), 3);
    }
};

QTEST_MAIN(TestBusClient)
#include "tst_busclient.moc"
