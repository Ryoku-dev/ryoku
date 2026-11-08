#include "buspath.hpp"
#include "bustestenv.hpp"
#include "busvalue.hpp"
#include "fakedaemon.hpp"

#include <QJSEngine>
#include <QJsonDocument>
#include <QSignalSpy>

namespace {

QJsonObject obj(const char *text)
{
    return QJsonDocument::fromJson(text).object();
}

}

class TestBusValue : public QObject
{
    Q_OBJECT

private slots:
    void splitForms()
    {
        QJSEngine engine;
        QCOMPARE(buspath::split(QJSValue("bar.style")), (QStringList{"bar", "style"}));
        QCOMPARE(buspath::split(engine.evaluate("['bar', 'style']")), (QStringList{"bar", "style"}));
        QCOMPARE(buspath::split(QJSValue("")), QStringList());
        QCOMPARE(buspath::split(QJSValue()), QStringList());
        QCOMPARE(buspath::split(QVariant(QStringList{"a"})), QStringList{"a"});
    }

    void touches()
    {
        const QStringList path{"bar", "style"};
        const QList<QStringList> exact{{"bar", "style"}};
        const QList<QStringList> ancestor{{"bar"}};
        const QList<QStringList> inside{{"bar", "style", "x"}};
        const QList<QStringList> sibling{{"bar", "size"}, {"dock"}};
        const QList<QStringList> root{{}};
        QVERIFY(buspath::touches(path, nullptr));
        QVERIFY(buspath::touches(path, &exact));
        QVERIFY(buspath::touches(path, &ancestor));
        QVERIFY(buspath::touches(path, &inside));
        QVERIFY(!buspath::touches(path, &sibling));
        QVERIFY(buspath::touches(path, &root));
        const QList<QStringList> none;
        QVERIFY(!buspath::touches(path, &none));
    }

    void at()
    {
        const QJsonValue root = obj(R"({"a":{"b":[10,{"c":null}]}})");
        bool found = false;
        QCOMPARE(buspath::at(root, {"a", "b", "0"}, &found), QJsonValue(10));
        QVERIFY(found);
        QCOMPARE(buspath::at(root, {"a", "b", "1", "c"}, &found), QJsonValue(QJsonValue::Null));
        QVERIFY(found);
        buspath::at(root, {"a", "x"}, &found);
        QVERIFY(!found);
        buspath::at(root, {"a", "b", "5"}, &found);
        QVERIFY(!found);
        buspath::at(root, {"a", "b", "0", "z"}, &found);
        QVERIFY(!found);
        QCOMPARE(buspath::at(root, {}, &found), root);
        QVERIFY(found);
    }

    void onlyMatchingPathChanges()
    {
        BusTestEnv env;
        FakeDaemon daemon(env.dir.path());
        QVERIFY(daemon.listen());
        QQmlEngine engine;
        BusTestEnv::prepare(engine);
        std::unique_ptr<QObject> style(BusTestEnv::create(engine, R"(BusValue { topic: "settings"; path: "bar.style"; fallback: "none" })"));
        std::unique_ptr<QObject> size(BusTestEnv::create(engine, R"(BusValue { topic: "settings"; path: ["bar", "size"] })"));
        std::unique_ptr<QObject> bar(BusTestEnv::create(engine, R"(BusValue { topic: "settings"; path: "bar" })"));
        QVERIFY(style && size && bar);
        QCOMPARE(style->property("value").value<QJSValue>().toString(), QStringLiteral("none"));
        QVERIFY(!style->property("present").toBool());
        QSignalSpy styleSpy(style.get(), SIGNAL(valueChanged()));
        QSignalSpy sizeSpy(size.get(), SIGNAL(valueChanged()));
        QSignalSpy barSpy(bar.get(), SIGNAL(valueChanged()));

        QTRY_COMPARE(daemon.ops("sub").size(), 1);
        QCOMPARE(daemon.ops("sub").first().value("topics").toArray(), QJsonArray{"settings"});
        daemon.send(obj(R"({"op":"hello","v":1})"));
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":1,"data":{"bar":{"style":"orpheus","size":3},"dock":{"n":1}}})"));
        QTRY_COMPARE(styleSpy.size(), 1);
        QCOMPARE(sizeSpy.size(), 1);
        QCOMPARE(barSpy.size(), 1);
        QCOMPARE(style->property("value").value<QJSValue>().toString(), QStringLiteral("orpheus"));
        QCOMPARE(size->property("value").value<QJSValue>().toInt(), 3);

        daemon.send(obj(R"({"op":"state","topic":"settings","rev":2,"base":1,"patch":[{"p":["bar","size"],"v":4}]})"));
        QTRY_COMPARE(sizeSpy.size(), 2);
        QCOMPARE(styleSpy.size(), 1);
        QCOMPARE(barSpy.size(), 2);

        daemon.send(obj(R"({"op":"state","topic":"settings","rev":3,"base":2,"patch":[{"p":["dock","n"],"v":2}]})"));
        // a full frame equal at the path changes nothing either
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":4,"data":{"bar":{"style":"orpheus","size":4},"dock":{"n":9}}})"));
        QTest::qWait(100);
        QCOMPARE(styleSpy.size(), 1);
        QCOMPARE(sizeSpy.size(), 2);
        QCOMPARE(barSpy.size(), 2);

        // a replaced ancestor moves the value, a delete brings the fallback back
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":5,"base":4,"patch":[{"p":["bar"],"v":{"style":"kairos"}}]})"));
        QTRY_COMPARE(styleSpy.size(), 2);
        QCOMPARE(style->property("value").value<QJSValue>().toString(), QStringLiteral("kairos"));
        QCOMPARE(sizeSpy.size(), 3);
        QVERIFY(size->property("value").value<QJSValue>().isUndefined());
        daemon.send(obj(R"({"op":"state","topic":"settings","rev":6,"base":5,"patch":[{"p":["bar","style"],"x":true}]})"));
        QTRY_COMPARE(styleSpy.size(), 3);
        QCOMPARE(style->property("value").value<QJSValue>().toString(), QStringLiteral("none"));

        // a new path reads the cached frame at once
        bar->setProperty("path", QVariant::fromValue(QJSValue("dock.n")));
        QCOMPARE(bar->property("value").value<QJSValue>().toInt(), 9);
    }
};

QTEST_MAIN(TestBusValue)
#include "tst_busvalue.moc"
