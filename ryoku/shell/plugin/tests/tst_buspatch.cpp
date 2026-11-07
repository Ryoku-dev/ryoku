#include "buspatch.hpp"

#include <QJSEngine>
#include <QJsonDocument>
#include <QJsonObject>
#include <QTest>

namespace {

QJsonValue json(const char *text)
{
    const QJsonDocument d = QJsonDocument::fromJson(text);
    return d.isObject() ? QJsonValue(d.object()) : QJsonValue(d.array());
}

QJsonArray ops(const char *text)
{
    return QJsonDocument::fromJson(text).array();
}

}

class TestBusPatch : public QObject
{
    Q_OBJECT

private slots:
    void setAndDelete()
    {
        const QJsonValue root = json(R"({"bar":{"style":"orpheus","size":3},"language":"en"})");
        const QJsonValue next = buspatch::apply(root, ops(R"([{"p":["bar","style"],"v":"kairos"},{"p":["language"],"x":true}])"));
        QCOMPARE(next, json(R"({"bar":{"style":"kairos","size":3}})"));
        // the input is untouched
        QCOMPARE(root, json(R"({"bar":{"style":"orpheus","size":3},"language":"en"})"));
    }

    void setCreatesMissingSteps()
    {
        const QJsonValue next = buspatch::apply(json(R"({"a":5})"), ops(R"([{"p":["a","b","c"],"v":[1,2]}])"));
        QCOMPARE(next, json(R"({"a":{"b":{"c":[1,2]}}})"));
    }

    void deleteThroughMissingStepDoesNothing()
    {
        const QJsonValue root = json(R"({"a":{"b":1}})");
        QCOMPARE(buspatch::apply(root, ops(R"([{"p":["x","y"],"x":true}])")), root);
        QCOMPARE(buspatch::apply(root, ops(R"([{"p":["a","b","c"],"x":true}])")), root);
    }

    void arraysReplacedWhole()
    {
        const QJsonValue next = buspatch::apply(json(R"({"list":[1,2,3]})"), ops(R"([{"p":["list"],"v":[4]}])"));
        QCOMPARE(next, json(R"({"list":[4]})"));
    }

    void emptyPathReplacesRoot()
    {
        QCOMPARE(buspatch::apply(json(R"({"a":1})"), ops(R"([{"p":[],"v":{"b":2}}])")), json(R"({"b":2})"));
        QCOMPARE(buspatch::apply(json(R"({"a":1})"), ops(R"([{"p":[],"x":true}])")), json("{}"));
        QCOMPARE(buspatch::apply(json(R"({"a":1})"), ops(R"([{"p":[],"v":3}])")), json("{}"));
    }

    void malformedOpsSkipped()
    {
        const QJsonValue root = json(R"({"a":1})");
        QCOMPARE(buspatch::apply(root, ops(R"([{"v":2},{"p":"a","v":3},5])")), root);
    }

    void numericKeys()
    {
        const QJsonValue next = buspatch::apply(json("{}"), ops(R"([{"p":["m",7],"v":true}])"));
        QCOMPARE(next, json(R"({"m":{"7":true}})"));
        QCOMPARE(buspatch::paths(ops(R"([{"p":["m",7],"v":true},{"p":["a"],"x":true}])")),
                 (QList<QStringList>{{"m", "7"}, {"a"}}));
    }

    // The JS copy: untouched subtrees keep their identity.
    void jsKeepsUntouchedIdentity()
    {
        QJSEngine engine;
        const QJSValue root = engine.toScriptValue(json(R"({"bar":{"style":"o"},"dock":{"size":2},"list":[1]})"));
        const QJSValue next = buspatch::apply(&engine, root, ops(R"([{"p":["bar","style"],"v":"k"},{"p":["bar","size"],"v":4}])"));
        QVERIFY(!next.strictlyEquals(root));
        QVERIFY(next.property("dock").strictlyEquals(root.property("dock")));
        QVERIFY(next.property("list").strictlyEquals(root.property("list")));
        QVERIFY(!next.property("bar").strictlyEquals(root.property("bar")));
        QCOMPARE(next.property("bar").property("style").toString(), QStringLiteral("k"));
        QCOMPARE(next.property("bar").property("size").toInt(), 4);
        QCOMPARE(root.property("bar").property("style").toString(), QStringLiteral("o"));
        QCOMPARE(engine.fromScriptValue<QJsonValue>(next), json(R"({"bar":{"style":"k","size":4},"dock":{"size":2},"list":[1]})"));
    }

    void jsMatchesJson()
    {
        QJSEngine engine;
        const QJsonValue root = json(R"({"a":{"b":1,"c":{"d":2}},"e":[1,2],"f":"x"})");
        const QJsonArray patch = ops(R"([{"p":["a","c","d"],"x":true},{"p":["e"],"v":{"z":1}},{"p":["g","h"],"v":null},{"p":["q","r"],"x":true},{"p":["f"],"x":true}])");
        const QJsonValue want = buspatch::apply(root, patch);
        const QJSValue got = buspatch::apply(&engine, engine.toScriptValue(root), patch);
        QCOMPARE(engine.fromScriptValue<QJsonValue>(got), want);
    }
};

QTEST_GUILESS_MAIN(TestBusPatch)
#include "tst_buspatch.moc"
