#pragma once

#include "busframer.hpp"

#include <QByteArray>
#include <QHash>
#include <QJsonArray>
#include <QJsonObject>
#include <QJsonValue>
#include <QList>
#include <QObject>
#include <QMutex>
#include <QSet>
#include <QString>

class QLocalSocket;
class QTimer;

// One message from the worker to the GUI thread, already parsed.
struct BusInbound {
    enum Kind { Opened, Closed, Hello, State, Event, Reply, Cmd, Error };
    Kind kind = Error;
    QString topic;
    qint64 rev = 0;
    qint64 seq = 0;
    qint64 dropped = 0;
    // State: the topic's whole frame after any patch. Event: the event.
    QJsonValue data;
    // State: the delta's ops when the frame came as a patch, else empty.
    QJsonArray patch;
    bool delta = false;
    // Reply, Cmd and Error: the message itself.
    QJsonObject msg;
};

// The socket side of the bus, on its own thread: connect and reconnect with
// backoff, the handshake with re-sub and re-serve, line framing, JSON parsing
// and delta frames applied to the topic's copy with a resync on a rev gap.
// What it reads waits in a hand-off queue the GUI thread drains with take()
// after each ready(). While the GUI thread is busy, state frames of one topic
// coalesce into the newest (events, replies and commands never do), as the
// daemon coalesces for a connection that reads slowly. Every slot runs on the
// worker thread; take() runs on the GUI thread.
class BusWorker : public QObject
{
    Q_OBJECT

public:
    static constexpr int BackoffMin = 250;
    static constexpr int BackoffMax = 5000;
    static int backoff(int attempt);

    BusWorker(QString socketPath, QString client, qint64 pid);
    ~BusWorker() override;

    // Worker thread only.
    void start();
    void shutdown();
    void addTopic(const QString &topic);
    void removeTopic(const QString &topic);
    void addTarget(const QString &target);
    void removeTarget(const QString &target);
    // Requests and replies; dropped when the socket is not up (the GUI side
    // fails what it had in flight when it sees Closed).
    void write(const QByteArray &bytes);

    // GUI thread: everything queued since the last take.
    QList<BusInbound> take();
    // Adds in to a hand-off queue, folding a state frame into a queued one of
    // the same topic (since the last Opened or Closed).
    static void enqueue(QList<BusInbound> &queue, BusInbound in);

signals:
    // Something is waiting for take(); not sent again until it is taken.
    void ready();

private:
    struct Cache {
        bool has = false;
        bool resync = false;
        qint64 rev = 0;
        QJsonValue data;
    };

    void dial();
    void opened();
    void down();
    void readable();
    void parse(const QByteArray &line, QList<BusInbound> &out);
    void state(const QJsonObject &msg, QList<BusInbound> &out);
    void send(const QByteArray &bytes);
    void deliver(QList<BusInbound> batch);

    QString m_path;
    QString m_client;
    qint64 m_pid;
    QLocalSocket *m_sock = nullptr;
    QTimer *m_retry = nullptr;
    BusFramer m_framer;
    bool m_open = false;
    int m_attempt = 0;
    QHash<QString, Cache> m_topics;
    QSet<QString> m_targets;
    QMutex m_lock;
    QList<BusInbound> m_pending;
    bool m_posted = false;
};
