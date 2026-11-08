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

struct BusInbound {
    enum Kind { Opened, Closed, Hello, State, Event, Reply, Cmd, Error };
    Kind kind = Error;
    QString topic;
    qint64 rev = 0;
    qint64 seq = 0;
    qint64 dropped = 0;
    QJsonValue data;
    QJsonArray patch;
    bool delta = false;
    QJsonObject msg;
};

// every slot runs on the worker thread, only take() runs on the GUI thread
class BusWorker : public QObject
{
    Q_OBJECT

public:
    static constexpr int BackoffMin = 250;
    static constexpr int BackoffMax = 5000;
    static int backoff(int attempt);

    BusWorker(QString socketPath, QString client, qint64 pid);
    ~BusWorker() override;

    // worker thread only
    void start();
    void shutdown();
    void addTopic(const QString &topic);
    void removeTopic(const QString &topic);
    void addTarget(const QString &target);
    void removeTarget(const QString &target);
    // dropped while the socket is down, the GUI side fails what it had in flight on Closed
    void write(const QByteArray &bytes);

    // GUI thread
    QList<BusInbound> take();
    // coalesces state frames of one topic like the daemon does for a slow reader
    static void enqueue(QList<BusInbound> &queue, BusInbound in);

signals:
    // not sent again until take()
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
