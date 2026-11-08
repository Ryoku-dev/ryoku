#include "buslistmodel.hpp"
#include "bustestenv.hpp"
#include "fakedaemon.hpp"

#include <QAbstractItemModelTester>
#include <QJsonDocument>
#include <QSignalSpy>

namespace {

struct Spies {
    explicit Spies(QAbstractItemModel *m)
        : reset(m, &QAbstractItemModel::modelReset)
        , inserted(m, &QAbstractItemModel::rowsInserted)
        , removed(m, &QAbstractItemModel::rowsRemoved)
        , moved(m, &QAbstractItemModel::rowsMoved)
        , changed(m, &QAbstractItemModel::dataChanged)
    {
    }
    void clear()
    {
        reset.clear();
        inserted.clear();
        removed.clear();
        moved.clear();
        changed.clear();
    }
    int structural() const { return int(reset.size() + inserted.size() + removed.size() + moved.size()); }
    QSignalSpy reset, inserted, removed, moved, changed;
};

int role(QAbstractItemModel *m, const char *name)
{
    return m->roleNames().key(name, -1);
}

QStringList column(QAbstractItemModel *m, const char *name)
{
    QStringList out;
    const int r = role(m, name);
    for (int i = 0; i < m->rowCount(); ++i)
        out.append(m->data(m->index(i, 0), r).toString());
    return out;
}

}

class TestBusListModel : public QObject
{
    Q_OBJECT

    std::unique_ptr<BusTestEnv> env;
    std::unique_ptr<FakeDaemon> daemon;
    std::unique_ptr<QQmlEngine> engine;
    int rev = 0;

    void frame(const char *windows)
    {
        ++rev;
        daemon->sendRaw(QStringLiteral(R"({"op":"state","topic":"wm","rev":%1,"data":{"focus":%1,"windows":%2}})" "\n")
                            .arg(rev)
                            .arg(QLatin1String(windows))
                            .toUtf8());
    }

    bool showsIds(QAbstractItemModel *m, const QStringList &ids)
    {
        return QTest::qWaitFor([&] { return column(m, "id") == ids; }, 3000);
    }

    BusListModel *model(const QByteArray &body)
    {
        QObject *o = BusTestEnv::create(*engine, body);
        return qobject_cast<BusListModel *>(o);
    }

private slots:
    void initTestCase()
    {
        // QSignalSpy copies the rows* arguments by metatype id
        qRegisterMetaType<QModelIndex>();
    }

    void init()
    {
        env = std::make_unique<BusTestEnv>();
        daemon = std::make_unique<FakeDaemon>(env->dir.path());
        QVERIFY(daemon->listen());
        engine = std::make_unique<QQmlEngine>();
        BusTestEnv::prepare(*engine);
        rev = 0;
    }

    void cleanup()
    {
        engine.reset();
        daemon.reset();
        env.reset();
    }

    void keyedDiff()
    {
        std::unique_ptr<BusListModel> m(model(R"(BusListModel { topic: "wm"; path: "windows"; key: "id" })"));
        QVERIFY(m);
        QAbstractItemModelTester tester(m.get(), QAbstractItemModelTester::FailureReportingMode::QtTest);
        Spies s(m.get());
        QTRY_COMPARE(daemon->ops("sub").size(), 1);
        daemon->send(QJsonObject{{"op", "hello"}, {"v", 1}});

        frame(R"([{"id":"a","title":"A","x":1},{"id":"b","title":"B","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "b", "c"}));
        QCOMPARE(m->count(), 3);
        QCOMPARE(s.reset.size(), 0);
        QCOMPARE(s.inserted.size(), 1);
        QVERIFY(m->roleNames().values().contains("title"));
        QVERIFY(m->roleNames().values().contains("modelData"));
        s.clear();

        // a changed row gets one dataChanged with only the roles that moved
        frame(R"([{"id":"a","title":"A","x":1},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QTRY_COMPARE(s.changed.size(), 1);
        QCOMPARE(s.structural(), 0);
        QCOMPARE(s.changed.first().at(0).toModelIndex().row(), 1);
        const QList<int> roles = s.changed.first().at(2).value<QList<int>>();
        QCOMPARE(roles.size(), 2);
        QVERIFY(roles.contains(role(m.get(), "title")));
        QVERIFY(roles.contains(role(m.get(), "modelData")));
        s.clear();

        frame(R"([{"id":"a","title":"A","x":1},{"id":"n","title":"N"},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "n", "b", "c"}));
        QCOMPARE(s.inserted.size(), 1);
        QCOMPARE(s.inserted.first().at(1).toInt(), 1);
        QCOMPARE(s.structural(), 1);
        QCOMPARE(s.changed.size(), 0);
        s.clear();

        frame(R"([{"id":"a","title":"A","x":1},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "b", "c"}));
        QCOMPARE(s.removed.size(), 1);
        QCOMPARE(s.structural(), 1);
        s.clear();

        // moving the last row to the front is one move, nothing else
        frame(R"([{"id":"c","title":"C","x":3},{"id":"a","title":"A","x":1},{"id":"b","title":"B2","x":2}])");
        QVERIFY(showsIds(m.get(), {"c", "a", "b"}));
        QCOMPARE(s.moved.size(), 1);
        QCOMPARE(s.structural(), 1);
        QCOMPARE(s.changed.size(), 0);
        s.clear();

        frame(R"([{"id":"a","title":"A","x":9},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "b", "c"}));
        QCOMPARE(s.moved.size(), 1);
        QTRY_COMPARE(s.changed.size(), 1);
        QCOMPARE(m->data(m->index(0, 0), role(m.get(), "x")).toInt(), 9);
        s.clear();

        // duplicate keys are told apart by order so no reset
        frame(R"([{"id":"a","title":"A","x":9},{"id":"a","title":"A'"},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "a", "b", "c"}));
        QCOMPARE(s.reset.size(), 0);
        QCOMPARE(s.inserted.size(), 1);
        QCOMPARE(s.inserted.first().at(1).toInt(), 1);
        s.clear();
        frame(R"([{"id":"a","title":"A'"},{"id":"b","title":"B2","x":2},{"id":"c","title":"C","x":3}])");
        QVERIFY(showsIds(m.get(), {"a", "b", "c"}));
        QCOMPARE(s.reset.size(), 0);
        QCOMPARE(s.removed.size(), 1);
        QCOMPARE(s.removed.first().at(1).toInt(), 1);
        QCOMPARE(s.changed.size(), 1);
        s.clear();

        // a row without the key resets
        frame(R"([{"id":"a","title":"A'"},{"title":"keyless"}])");
        QTRY_COMPARE(s.reset.size(), 1);
        QCOMPARE(m->count(), 2);
        s.clear();
        frame(R"([{"id":"a","title":"A'"},{"id":"z"}])");
        QVERIFY(showsIds(m.get(), {"a", "z"}));
        s.clear();

        QCOMPARE(m->indexOf(QStringLiteral("z")), 1);
        QCOMPARE(m->indexOf(QStringLiteral("q")), -1);
        QCOMPARE(engine->fromScriptValue<QJsonValue>(m->get(0)), QJsonValue(QJsonObject{{"id", "a"}, {"title", "A'"}}));
        QVERIFY(m->get(5).isUndefined());
    }

    void numericKeysAndPatchSkip()
    {
        std::unique_ptr<BusListModel> m(model(R"(BusListModel { topic: "wm"; path: "windows" })"));
        Spies s(m.get());
        QTRY_COMPARE(daemon->ops("sub").size(), 1);
        daemon->send(QJsonObject{{"op", "hello"}, {"v", 1}});
        daemon->sendRaw(R"({"op":"state","topic":"wm","rev":1,"data":{"focus":0,"windows":[{"id":1},{"id":2}]}})" "\n");
        QTRY_COMPARE(m->count(), 2);
        QCOMPARE(m->indexOf(2), 1);
        QCOMPARE(m->indexOf(QStringLiteral("2")), -1);
        s.clear();
        // a patch elsewhere in the frame is skipped without a diff
        daemon->sendRaw(R"({"op":"state","topic":"wm","rev":2,"base":1,"patch":[{"p":["focus"],"v":5}]})" "\n");
        daemon->sendRaw(R"({"op":"state","topic":"wm","rev":3,"base":2,"patch":[{"p":["windows"],"v":[{"id":2},{"id":1}]}]})" "\n");
        QTRY_COMPARE(s.moved.size(), 1);
        QCOMPARE(s.structural(), 1);
        QCOMPARE(m->indexOf(2), 0);
    }

    void filterAndExplicitRoles()
    {
        std::unique_ptr<BusListModel> m(model(R"(BusListModel {
            topic: "wm"; path: "windows"; filterRole: "ws"; filterValue: 1
            roles: ["id", "title"]
        })"));
        Spies s(m.get());
        QTRY_COMPARE(daemon->ops("sub").size(), 1);
        daemon->send(QJsonObject{{"op", "hello"}, {"v", 1}});
        frame(R"([{"id":"a","ws":1,"title":"A","x":1},{"id":"b","ws":2,"title":"B"},{"id":"c","ws":1,"title":"C"}])");
        QVERIFY(showsIds(m.get(), {"a", "c"}));
        QCOMPARE(m->roleNames().size(), 3);
        QVERIFY(!m->data(m->index(0, 0), role(m.get(), "modelData")).value<QJsonObject>().contains("x"));
        s.clear();
        // only a non-role key moved so nothing happens
        frame(R"([{"id":"a","ws":1,"title":"A","x":2},{"id":"b","ws":2,"title":"B"},{"id":"c","ws":1,"title":"C"}])");
        QTest::qWait(100);
        QCOMPARE(s.structural(), 0);
        QCOMPARE(s.changed.size(), 0);
        QCOMPARE(engine->fromScriptValue<QJsonObject>(m->get(0)).value("x").toInt(), 2);
        // a row leaves the filter and another joins
        frame(R"([{"id":"a","ws":2,"title":"A"},{"id":"b","ws":1,"title":"B"},{"id":"c","ws":1,"title":"C"}])");
        QVERIFY(showsIds(m.get(), {"b", "c"}));
        QCOMPARE(s.reset.size(), 0);
        // changing the filter re-diffs the held frame
        s.clear();
        m->setFilterValue(2);
        QVERIFY(showsIds(m.get(), {"a"}));
        QCOMPARE(s.reset.size(), 0);
    }

    void topicOffClears()
    {
        std::unique_ptr<BusListModel> m(model(R"(BusListModel { topic: "wm"; path: "windows" })"));
        QTRY_COMPARE(daemon->ops("sub").size(), 1);
        daemon->send(QJsonObject{{"op", "hello"}, {"v", 1}});
        frame(R"([{"id":"a"},{"id":"b"}])");
        QTRY_COMPARE(m->count(), 2);
        Spies s(m.get());
        m->setTopic(QString());
        QCOMPARE(m->count(), 0);
        QCOMPARE(s.removed.size(), 1);
        QCOMPARE(s.reset.size(), 0);
        QTRY_COMPARE(daemon->ops("unsub").size(), 1);
    }

    void localRows()
    {
        std::unique_ptr<QObject> holder(BusTestEnv::create(*engine, R"(QtObject {
            property QtObject one: QtObject {}
            property QtObject two: QtObject {}
            property BusListModel model: BusListModel { roles: ["id", "handle", "tags"] }
        })"));
        QVERIFY(holder);
        QQmlEngine::setObjectOwnership(holder.get(), QQmlEngine::CppOwnership);
        auto *m = holder->property("model").value<BusListModel *>();
        QVERIFY(m);
        QAbstractItemModelTester tester(m, QAbstractItemModelTester::FailureReportingMode::QtTest);
        Spies s(m);
        QJSValue rows = engine->evaluate("(function (h) { return [{id: 1, handle: h.one, tags: ['x']}, {id: 2, handle: null, tags: []}]; })")
                            .call({engine->newQObject(holder.get())});
        m->setRows(rows);
        QCOMPARE(m->count(), 2);
        QCOMPARE(m->data(m->index(0, 0), role(m, "handle")).value<QObject *>(), holder->property("one").value<QObject *>());
        s.clear();
        // same values in a new array do nothing, a new handle or tag changes that role
        m->setRows(engine->evaluate("(function (h) { return [{id: 1, handle: h.one, tags: ['x']}, {id: 2, handle: h.two, tags: []}]; })")
                       .call({engine->newQObject(holder.get())}));
        QCOMPARE(s.structural(), 0);
        QCOMPARE(s.changed.size(), 1);
        QCOMPARE(s.changed.first().at(0).toModelIndex().row(), 1);
        QVERIFY(s.changed.first().at(2).value<QList<int>>().contains(role(m, "handle")));
        s.clear();
        m->setRows(engine->evaluate("(function (h) { return [{id: 2, handle: h.two, tags: ['y']}, {id: 1, handle: h.one, tags: ['x']}]; })")
                       .call({engine->newQObject(holder.get())}));
        QCOMPARE(s.moved.size(), 1);
        QCOMPARE(s.changed.size(), 1);
        QCOMPARE(m->indexOf(2), 0);
        QCOMPARE(m->get(0).property("tags").property(0).toString(), QStringLiteral("y"));
    }
};

QTEST_MAIN(TestBusListModel)
#include "tst_buslistmodel.moc"
