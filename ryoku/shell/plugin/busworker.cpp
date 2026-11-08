#include "busworker.hpp"

#include "buspatch.hpp"

#include <QJsonDocument>
#include <QLocalSocket>
#include <QTimer>

namespace {

constexpr int Version = 1;

QByteArray encode(const QJsonObject &msg)
{
    QByteArray out = QJsonDocument(msg).toJson(QJsonDocument::Compact);
    out.append('\n');
    return out;
}

QByteArray topicsLine(const char *op, const QStringList &names)
{
    return encode({{QStringLiteral("op"), QLatin1String(op)},
                   {QStringLiteral("topics"), QJsonArray::fromStringList(names)}});
}

QByteArray targetsLine(const char *op, const QStringList &names)
{
    return encode({{QStringLiteral("op"), QLatin1String(op)},
                   {QStringLiteral("targets"), QJsonArray::fromStringList(names)}});
}

}

int BusWorker::backoff(int attempt)
{
    const int shift = qBound(0, attempt, 16);
    return int(qMin<qint64>(BackoffMax, qint64(BackoffMin) << shift));
}

BusWorker::BusWorker(QString socketPath, QString client, qint64 pid)
    : m_path(std::move(socketPath))
    , m_client(std::move(client))
    , m_pid(pid)
{
}

BusWorker::~BusWorker() = default;

void BusWorker::start()
{
    m_sock = new QLocalSocket(this);
    m_retry = new QTimer(this);
    m_retry->setSingleShot(true);
    connect(m_retry, &QTimer::timeout, this, &BusWorker::dial);
    connect(m_sock, &QLocalSocket::connected, this, &BusWorker::opened);
    connect(m_sock, &QLocalSocket::readyRead, this, &BusWorker::readable);
    // a failed dial only reports an error, a dropped peer reports the error and then the state change
    connect(m_sock, &QLocalSocket::stateChanged, this, [this](QLocalSocket::LocalSocketState s) {
        if (s == QLocalSocket::UnconnectedState)
            down();
    });
    connect(m_sock, &QLocalSocket::errorOccurred, this, [this] {
        if (m_sock->state() == QLocalSocket::UnconnectedState)
            down();
    });
    dial();
}

void BusWorker::shutdown()
{
    if (m_retry)
        m_retry->stop();
    if (m_sock) {
        m_sock->disconnect(this);
        m_sock->abort();
    }
    delete m_sock;
    delete m_retry;
    m_sock = nullptr;
    m_retry = nullptr;
    m_open = false;
}

void BusWorker::dial()
{
    if (!m_sock || m_sock->state() != QLocalSocket::UnconnectedState)
        return;
    m_sock->connectToServer(m_path);
}

// the daemon reads the connection in order so nothing waits for hello
void BusWorker::opened()
{
    m_retry->stop();
    m_open = true;
    m_framer.clear();
    QByteArray text = "bus ";
    text += QJsonDocument(QJsonObject{{QStringLiteral("v"), Version},
                                      {QStringLiteral("client"), m_client},
                                      {QStringLiteral("pid"), m_pid}})
                .toJson(QJsonDocument::Compact);
    text += '\n';
    QStringList live;
    live.reserve(m_topics.size());
    for (auto it = m_topics.begin(); it != m_topics.end(); ++it) {
        it->resync = false;
        live.append(it.key());
    }
    if (!live.isEmpty())
        text += topicsLine("sub", live);
    if (!m_targets.isEmpty())
        text += targetsLine("serve", QStringList(m_targets.begin(), m_targets.end()));
    send(text);
    BusInbound in;
    in.kind = BusInbound::Opened;
    deliver({in});
}

void BusWorker::down()
{
    if (!m_retry || m_retry->isActive())
        return;
    const bool wasOpen = m_open;
    m_open = false;
    m_framer.clear();
    m_retry->start(backoff(m_attempt++));
    if (wasOpen) {
        BusInbound in;
        in.kind = BusInbound::Closed;
        deliver({in});
    }
}

void BusWorker::send(const QByteArray &bytes)
{
    if (!m_sock || !m_open)
        return;
    m_sock->write(bytes);
    m_sock->flush();
}

void BusWorker::write(const QByteArray &bytes)
{
    send(bytes);
}

void BusWorker::addTopic(const QString &topic)
{
    if (m_topics.contains(topic))
        return;
    m_topics.insert(topic, Cache{});
    send(topicsLine("sub", {topic}));
}

void BusWorker::removeTopic(const QString &topic)
{
    if (m_topics.remove(topic))
        send(topicsLine("unsub", {topic}));
}

void BusWorker::addTarget(const QString &target)
{
    if (m_targets.contains(target))
        return;
    m_targets.insert(target);
    send(targetsLine("serve", {target}));
}

void BusWorker::removeTarget(const QString &target)
{
    if (m_targets.remove(target))
        send(targetsLine("unserve", {target}));
}

void BusWorker::readable()
{
    QList<QByteArray> lines;
    m_framer.append(m_sock->readAll(), lines);
    if (lines.isEmpty())
        return;
    QList<BusInbound> out;
    out.reserve(lines.size());
    for (const QByteArray &line : std::as_const(lines))
        parse(line, out);
    if (!out.isEmpty())
        deliver(std::move(out));
}

void BusWorker::deliver(QList<BusInbound> batch)
{
    QMutexLocker lock(&m_lock);
    for (BusInbound &in : batch)
        enqueue(m_pending, std::move(in));
    if (m_posted)
        return;
    m_posted = true;
    lock.unlock();
    emit ready();
}

QList<BusInbound> BusWorker::take()
{
    QMutexLocker lock(&m_lock);
    m_posted = false;
    return std::exchange(m_pending, {});
}

void BusWorker::enqueue(QList<BusInbound> &queue, BusInbound in)
{
    if (in.kind == BusInbound::State) {
        for (qsizetype j = queue.size() - 1; j >= 0; --j) {
            BusInbound &o = queue[j];
            if (o.kind == BusInbound::Opened || o.kind == BusInbound::Closed)
                break;
            if (o.kind != BusInbound::State || o.topic != in.topic)
                continue;
            // a delta on a queued delta keeps both patches so the JS copy applies them in order
            if (in.delta && o.delta) {
                for (const QJsonValue &op : std::as_const(in.patch))
                    o.patch.append(op);
                o.data = std::move(in.data);
                o.rev = in.rev;
            } else if (in.delta) {
                o.data = std::move(in.data);
                o.rev = in.rev;
            } else {
                o = std::move(in);
            }
            queue.move(j, queue.size() - 1);
            return;
        }
    }
    queue.append(std::move(in));
}

void BusWorker::parse(const QByteArray &line, QList<BusInbound> &out)
{
    const QJsonDocument doc = QJsonDocument::fromJson(line);
    if (!doc.isObject())
        return;
    const QJsonObject msg = doc.object();
    const QString op = msg.value(QLatin1String("op")).toString();
    BusInbound in;
    if (op == QLatin1String("hello")) {
        m_attempt = 0;
        in.kind = BusInbound::Hello;
    } else if (op == QLatin1String("state")) {
        state(msg, out);
        return;
    } else if (op == QLatin1String("event")) {
        in.kind = BusInbound::Event;
        in.topic = msg.value(QLatin1String("topic")).toString();
        if (!m_topics.contains(in.topic))
            return;
        in.seq = msg.value(QLatin1String("seq")).toInteger();
        in.dropped = msg.value(QLatin1String("dropped")).toInteger();
        in.data = msg.value(QLatin1String("data"));
    } else if (op == QLatin1String("reply")) {
        in.kind = BusInbound::Reply;
        in.msg = msg;
    } else if (op == QLatin1String("cmd")) {
        in.kind = BusInbound::Cmd;
        in.msg = msg;
    } else if (op == QLatin1String("error")) {
        in.kind = BusInbound::Error;
        in.msg = msg;
    } else {
        return;
    }
    out.append(std::move(in));
}

void BusWorker::state(const QJsonObject &msg, QList<BusInbound> &out)
{
    const QString topic = msg.value(QLatin1String("topic")).toString();
    const auto it = m_topics.find(topic);
    if (it == m_topics.end())
        return;
    Cache &t = *it;
    BusInbound in;
    in.kind = BusInbound::State;
    in.topic = topic;
    in.rev = msg.value(QLatin1String("rev")).toInteger();
    if (msg.contains(QLatin1String("data"))) {
        t.data = msg.value(QLatin1String("data"));
        t.rev = in.rev;
        t.has = true;
        t.resync = false;
        in.data = t.data;
        out.append(std::move(in));
        return;
    }
    const QJsonValue patch = msg.value(QLatin1String("patch"));
    if (!patch.isArray())
        return;
    if (!t.has || t.rev != msg.value(QLatin1String("base")).toInteger()) {
        if (!t.resync && m_open) {
            t.resync = true;
            send(topicsLine("sub", {topic}));
        }
        return;
    }
    in.patch = patch.toArray();
    in.delta = true;
    t.data = buspatch::apply(t.data, in.patch);
    t.rev = in.rev;
    in.data = t.data;
    out.append(std::move(in));
}
