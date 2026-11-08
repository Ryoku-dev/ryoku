#include "busclient.hpp"

#include "busdispatch.hpp"
#include "buspatch.hpp"

#include <QDebug>
#include <QCoreApplication>
#include <QJSEngine>
#include <QJsonArray>
#include <QJsonDocument>
#include <QQmlEngine>
#include <QThread>

namespace {

QByteArray encode(const QJsonObject &msg)
{
    QByteArray out = QJsonDocument(msg).toJson(QJsonDocument::Compact);
    out.append('\n');
    return out;
}

}

BusClient::BusClient(QJSEngine *engine, QObject *parent)
    : QObject(parent)
    , m_engine(engine)
{
}

BusClient::~BusClient()
{
    if (!m_thread)
        return;
    BusWorker *w = m_worker;
    QMetaObject::invokeMethod(w, [w] { w->shutdown(); }, Qt::BlockingQueuedConnection);
    m_thread->quit();
    m_thread->wait();
    delete m_worker;
    delete m_thread;
}

BusClient *BusClient::create(QQmlEngine *, QJSEngine *js)
{
    return new BusClient(js);
}

BusClient *BusClient::of(QObject *obj)
{
    QQmlEngine *engine = qmlEngine(obj);
    return engine ? engine->singletonInstance<BusClient *>("Ryoku.Bus", "BusClient") : nullptr;
}

QString BusClient::socketPath() const
{
    QString dir = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (dir.isEmpty())
        dir = QStringLiteral("/tmp");
    return dir + QStringLiteral("/ryoku-shell.sock");
}

// a dev checkout runs qs -p ryoku/hub/quickshell so quickshell stands for its parent
QString BusClient::clientName() const
{
    if (!m_clientName.isEmpty())
        return m_clientName;
    QString name = qEnvironmentVariable("RYOKU_BUS_CLIENT");
    if (name.isEmpty()) {
        QString dir;
        if (auto *qml = qobject_cast<QQmlEngine *>(m_engine)) {
            if (QObject *qs = qml->singletonInstance<QObject *>("Quickshell", "Quickshell"))
                dir = qs->property("shellDir").toString();
        }
        const QStringList parts = dir.split(QLatin1Char('/'), Qt::SkipEmptyParts);
        if (!parts.isEmpty()) {
            name = parts.last();
            if (name == QLatin1String("quickshell") && parts.size() > 1)
                name = parts.at(parts.size() - 2);
        }
        if (name.isEmpty())
            name = QStringLiteral("shell");
    }
    m_clientName = name;
    return name;
}

// opened on first use so a process that never touches the bus never dials
void BusClient::ensure()
{
    if (m_thread)
        return;
    m_thread = new QThread();
    m_thread->setObjectName(QStringLiteral("ryoku-bus"));
    m_worker = new BusWorker(socketPath(), clientName(), QCoreApplication::applicationPid());
    m_worker->moveToThread(m_thread);
    connect(m_worker, &BusWorker::ready, this, [this] { receive(m_worker->take()); }, Qt::QueuedConnection);
    m_thread->start();
    post([](BusWorker *w) { w->start(); });
}

void BusClient::post(std::function<void(BusWorker *)> fn)
{
    BusWorker *w = m_worker;
    QMetaObject::invokeMethod(w, [w, fn = std::move(fn)] { fn(w); }, Qt::QueuedConnection);
}

int BusClient::subscribe(const QString &topic, const QJSValue &fn)
{
    ensure();
    auto it = m_topics.find(topic);
    if (it == m_topics.end()) {
        it = m_topics.insert(topic, Topic{});
        post([topic](BusWorker *w) { w->addTopic(topic); });
    }
    const int token = ++m_nextToken;
    it->subs.insert(token, fn);
    m_tokenTopic.insert(token, topic);
    if (it->has) {
        QJSValue info = m_engine->newObject();
        info.setProperty(QStringLiteral("rev"), double(it->rev));
        info.setProperty(QStringLiteral("paths"), QJSValue(QJSValue::NullValue));
        const QJSValue ret = QJSValue(fn).call({jsData(*it), info});
        if (ret.isError())
            qWarning().noquote() << "bus: topic subscriber failed:" << ret.toString();
    }
    return token;
}

void BusClient::unsubscribe(int token)
{
    const auto at = m_tokenTopic.find(token);
    if (at == m_tokenTopic.end())
        return;
    const QString name = at.value();
    m_tokenTopic.erase(at);
    const auto it = m_topics.find(name);
    if (it == m_topics.end() || !it->subs.remove(token))
        return;
    dropTopicIfIdle(name);
}

void BusClient::listen(const QString &topic, BusListener *listener)
{
    ensure();
    auto it = m_topics.find(topic);
    if (it == m_topics.end()) {
        it = m_topics.insert(topic, Topic{});
        post([topic](BusWorker *w) { w->addTopic(topic); });
    }
    if (!it->listeners.contains(listener))
        it->listeners.append(listener);
}

void BusClient::unlisten(const QString &topic, BusListener *listener)
{
    const auto it = m_topics.find(topic);
    if (it == m_topics.end() || !it->listeners.removeOne(listener))
        return;
    dropTopicIfIdle(topic);
}

bool BusClient::frame(const QString &topic, QJsonValue *data) const
{
    const auto it = m_topics.constFind(topic);
    if (it == m_topics.constEnd() || !it->has)
        return false;
    *data = it->data;
    return true;
}

void BusClient::dropTopicIfIdle(const QString &name)
{
    const auto it = m_topics.find(name);
    if (it == m_topics.end() || !it->subs.isEmpty() || !it->listeners.isEmpty())
        return;
    m_topics.erase(it);
    post([name](BusWorker *w) { w->removeTopic(name); });
}

QJSValue BusClient::jsData(Topic &t)
{
    if (!t.jsValid) {
        t.js = m_engine->toScriptValue(t.data);
        t.jsValid = true;
    }
    return t.js;
}

void BusClient::seed(const QString &topic, const QJSValue &data)
{
    const auto it = m_topics.find(topic);
    if (it == m_topics.end() || (m_connected && it->fromDaemon))
        return;
    it->data = m_engine->fromScriptValue<QJsonValue>(data);
    it->js = data;
    it->jsValid = true;
    it->has = true;
    it->fromDaemon = false;
    it->rev = 0;
    QJSValue info = m_engine->newObject();
    info.setProperty(QStringLiteral("rev"), 0);
    info.setProperty(QStringLiteral("paths"), QJSValue(QJSValue::NullValue));
    notify(topic, data, info, it->data, nullptr, false);
}

void BusClient::notify(const QString &name, const QJSValue &data, const QJSValue &info,
                       const QJsonValue &json, const QList<QStringList> *paths, bool event)
{
    // a subscriber may unsubscribe itself or others, or drop the topic
    auto it = m_topics.find(name);
    if (it == m_topics.end())
        return;
    const QList<int> tokens = it->subs.keys();
    for (int token : tokens) {
        it = m_topics.find(name);
        if (it == m_topics.end())
            return;
        const auto sub = it->subs.constFind(token);
        if (sub == it->subs.constEnd())
            continue;
        const QJSValue fn = sub.value();
        const QJSValue ret = fn.call({data, info});
        if (ret.isError())
            qWarning().noquote() << "bus: topic subscriber failed:" << ret.toString();
    }
    if (event)
        return;
    it = m_topics.find(name);
    if (it == m_topics.end())
        return;
    const QList<BusListener *> listeners = it->listeners;
    for (BusListener *l : listeners) {
        it = m_topics.find(name);
        if (it == m_topics.end())
            return;
        if (it->listeners.contains(l))
            l->busFrame(json, paths);
    }
}

int BusClient::request(Kind kind, QJsonObject msg, const QJSValue &cb)
{
    ensure();
    const int id = ++m_nextId;
    msg.insert(QStringLiteral("id"), id);
    Request req{id, kind, std::move(msg), cb};
    if (m_open) {
        sendNow(req);
        return id;
    }
    if (m_queue.size() >= QueueMax) {
        const Request dropped = m_queue.takeFirst();
        finish(dropped.kind, dropped.cb, false, QJSValue(QString()), QStringLiteral("queue full"));
    }
    m_queue.append(std::move(req));
    return id;
}

void BusClient::sendNow(const Request &req)
{
    m_inflight.insert(req.id, Pending{req.kind, req.cb});
    const QByteArray bytes = encode(req.msg);
    post([bytes](BusWorker *w) { w->write(bytes); });
}

void BusClient::finish(Kind kind, const QJSValue &cb, bool ok, const QJSValue &result, const QString &error)
{
    if (!cb.isCallable())
        return;
    QJSValue r = result;
    if (kind != Call && (r.isUndefined() || r.isNull()))
        r = QJSValue(QString());
    const QJSValue ret = QJSValue(cb).call({QJSValue(ok), r, QJSValue(error)});
    if (ret.isError())
        qWarning().noquote() << "bus: reply callback failed:" << ret.toString();
}

int BusClient::call(const QString &method, const QJSValue &args, const QJSValue &cb)
{
    QJsonObject msg{{QStringLiteral("op"), QStringLiteral("call")}, {QStringLiteral("method"), method}};
    msg.insert(QStringLiteral("args"), args.isUndefined() ? QJsonValue(QJsonObject()) : m_engine->fromScriptValue<QJsonValue>(args));
    return request(Call, std::move(msg), cb);
}

int BusClient::run(const QString &line, const QJSValue &cb, const QJSValue &stdinText)
{
    QJsonObject msg{{QStringLiteral("op"), QStringLiteral("run")}, {QStringLiteral("line"), line}};
    if (!stdinText.isUndefined() && !stdinText.isNull())
        msg.insert(QStringLiteral("stdin"), stdinText.toString());
    return request(Run, std::move(msg), cb);
}

int BusClient::ipc(const QString &target, const QString &fn, const QJSValue &args, const QJSValue &cb,
                   const QJSValue &client)
{
    QJsonArray list;
    if (args.toBool()) {
        const int n = args.property(QStringLiteral("length")).toInt();
        for (int i = 0; i < n; ++i)
            list.append(args.property(quint32(i)).toString());
    }
    QJsonObject msg{{QStringLiteral("op"), QStringLiteral("ipc")},
                    {QStringLiteral("target"), target},
                    {QStringLiteral("fn"), fn},
                    {QStringLiteral("args"), list}};
    if (client.toBool())
        msg.insert(QStringLiteral("client"), client.toString());
    return request(Ipc, std::move(msg), cb);
}

int BusClient::ipcGet(const QString &target, const QString &prop, const QJSValue &cb, const QJSValue &client)
{
    QJsonObject msg{{QStringLiteral("op"), QStringLiteral("ipc")},
                    {QStringLiteral("target"), target},
                    {QStringLiteral("prop"), prop}};
    if (client.toBool())
        msg.insert(QStringLiteral("client"), client.toString());
    return request(Ipc, std::move(msg), cb);
}

bool BusClient::cancel(int id)
{
    for (qsizetype i = 0; i < m_queue.size(); ++i) {
        if (m_queue.at(i).id == id) {
            m_queue.removeAt(i);
            return true;
        }
    }
    return false;
}

void BusClient::serve(const QString &target, const QJSValue &handler)
{
    ensure();
    QList<QJSValue> &list = m_targets[target];
    for (const QJSValue &h : std::as_const(list))
        if (h.strictlyEquals(handler))
            return;
    list.append(handler);
    if (list.size() == 1)
        post([target](BusWorker *w) { w->addTarget(target); });
}

void BusClient::unserve(const QString &target, const QJSValue &handler)
{
    const auto it = m_targets.find(target);
    if (it == m_targets.end())
        return;
    QList<QJSValue> &list = it.value();
    for (qsizetype i = 0; i < list.size(); ++i) {
        if (list.at(i).strictlyEquals(handler)) {
            list.removeAt(i);
            if (list.isEmpty()) {
                m_targets.erase(it);
                post([target](BusWorker *w) { w->removeTarget(target); });
            }
            return;
        }
    }
}

void BusClient::serveAs(const QJSValue &handler, const QString &target)
{
    if (target.isEmpty() && !m_thread)
        return;
    const QStringList names = m_targets.keys();
    for (const QString &name : names) {
        if (name == target)
            continue;
        const auto it = m_targets.constFind(name);
        if (it == m_targets.constEnd())
            continue;
        for (const QJSValue &h : it.value()) {
            if (h.strictlyEquals(handler)) {
                unserve(name, handler);
                break;
            }
        }
    }
    if (!target.isEmpty())
        serve(target, handler);
}

void BusClient::setConnected(bool v)
{
    if (m_connected == v)
        return;
    m_connected = v;
    emit connectedChanged();
}

void BusClient::receive(const QList<BusInbound> &batch)
{
    for (const BusInbound &in : batch) {
        switch (in.kind) {
        case BusInbound::Opened:
            onOpened();
            break;
        case BusInbound::Closed:
            onClosed();
            break;
        case BusInbound::Hello:
            setConnected(true);
            break;
        case BusInbound::State:
            onState(in);
            break;
        case BusInbound::Event:
            onEvent(in);
            break;
        case BusInbound::Reply:
            onReply(in.msg);
            break;
        case BusInbound::Cmd:
            onCmd(in.msg);
            break;
        case BusInbound::Error: {
            const QString topic = in.msg.value(QLatin1String("topic")).toString();
            qWarning().noquote().nospace() << "bus: " << (topic.isEmpty() ? QString() : topic + QStringLiteral(": "))
                                           << in.msg.value(QLatin1String("error")).toVariant().toString();
            break;
        }
        }
    }
}

void BusClient::onOpened()
{
    m_open = true;
    const QList<Request> pending = std::exchange(m_queue, {});
    if (pending.isEmpty())
        return;
    QByteArray bytes;
    for (const Request &req : pending) {
        m_inflight.insert(req.id, Pending{req.kind, req.cb});
        bytes += encode(req.msg);
    }
    post([bytes](BusWorker *w) { w->write(bytes); });
}

// the daemon may or may not have run what was in flight, so it fails and is never resent
void BusClient::onClosed()
{
    m_open = false;
    setConnected(false);
    for (auto it = m_topics.begin(); it != m_topics.end(); ++it)
        it->fromDaemon = false;
    const QHash<int, Pending> lost = std::exchange(m_inflight, {});
    QList<int> ids = lost.keys();
    std::sort(ids.begin(), ids.end());
    for (int id : ids) {
        const Pending &p = lost[id];
        finish(p.kind, p.cb, false, QJSValue(QString()), QStringLiteral("disconnected"));
    }
}

void BusClient::onState(const BusInbound &in)
{
    const auto it = m_topics.find(in.topic);
    if (it == m_topics.end())
        return;
    Topic &t = *it;
    const bool incremental = in.delta && t.fromDaemon && t.jsValid;
    t.data = in.data;
    t.rev = in.rev;
    t.has = true;
    t.fromDaemon = true;
    QList<QStringList> paths;
    if (in.delta)
        paths = buspatch::paths(in.patch);
    QJSValue data;
    QJSValue info;
    if (!t.subs.isEmpty()) {
        if (incremental) {
            t.js = buspatch::apply(m_engine, t.js, in.patch);
        } else {
            t.js = m_engine->toScriptValue(t.data);
        }
        t.jsValid = true;
        data = t.js;
        info = m_engine->newObject();
        info.setProperty(QStringLiteral("rev"), double(in.rev));
        if (in.delta) {
            QJSValue list = m_engine->newArray(uint(paths.size()));
            for (qsizetype i = 0; i < paths.size(); ++i)
                list.setProperty(quint32(i), m_engine->toScriptValue(paths.at(i)));
            info.setProperty(QStringLiteral("paths"), list);
        } else {
            info.setProperty(QStringLiteral("paths"), QJSValue(QJSValue::NullValue));
        }
    } else {
        t.js = QJSValue();
        t.jsValid = false;
    }
    notify(in.topic, data, info, in.data, in.delta ? &paths : nullptr, false);
}

void BusClient::onEvent(const BusInbound &in)
{
    const auto it = m_topics.find(in.topic);
    if (it == m_topics.end() || it->subs.isEmpty())
        return;
    QJSValue info = m_engine->newObject();
    info.setProperty(QStringLiteral("seq"), double(in.seq));
    info.setProperty(QStringLiteral("dropped"), double(in.dropped));
    info.setProperty(QStringLiteral("event"), true);
    notify(in.topic, m_engine->toScriptValue(in.data), info, {}, nullptr, true);
}

void BusClient::onReply(const QJsonObject &msg)
{
    const int id = msg.value(QLatin1String("id")).toInt();
    const auto it = m_inflight.find(id);
    if (it == m_inflight.end())
        return;
    const Pending p = it.value();
    m_inflight.erase(it);
    const bool ok = msg.value(QLatin1String("ok")) == QJsonValue(true);
    const QJsonValue result = msg.value(QLatin1String("result"));
    QString error;
    if (!ok) {
        const QJsonValue e = msg.value(QLatin1String("error"));
        error = e.isString() && !e.toString().isEmpty() ? e.toString() : QStringLiteral("failed");
    }
    finish(p.kind, p.cb, ok, m_engine->toScriptValue(result), error);
}

void BusClient::onCmd(const QJsonObject &msg)
{
    const QString target = msg.value(QLatin1String("target")).toString();
    const auto it = m_targets.constFind(target);
    const QJSValue handler = it != m_targets.constEnd() && !it->isEmpty() ? it->last() : QJSValue();
    QJsonObject done = busdispatch::run(m_engine, handler, msg);
    done.insert(QStringLiteral("op"), QStringLiteral("done"));
    done.insert(QStringLiteral("id"), msg.value(QLatin1String("id")));
    if (!m_open)
        return;
    const QByteArray bytes = encode(done);
    post([bytes](BusWorker *w) { w->write(bytes); });
}
