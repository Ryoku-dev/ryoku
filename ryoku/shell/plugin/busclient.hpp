#pragma once

#include "busworker.hpp"

#include <functional>

#include <QHash>
#include <QJSValue>
#include <QJsonValue>
#include <QList>
#include <QMap>
#include <QObject>
#include <QPointer>
#include <QStringList>
#include <QtQml/qqmlregistration.h>

class QJSEngine;
class QQmlEngine;
class QThread;

class BusListener
{
public:
    virtual ~BusListener() = default;
    virtual void busFrame(const QJsonValue &data, const QList<QStringList> *paths) = 0;
};

class BusClient : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON
    Q_PROPERTY(bool connected READ connected NOTIFY connectedChanged)
    Q_PROPERTY(QString socketPath READ socketPath CONSTANT)
    Q_PROPERTY(QString clientName READ clientName CONSTANT)

public:
    static constexpr int QueueMax = 256;

    explicit BusClient(QJSEngine *engine, QObject *parent = nullptr);
    ~BusClient() override;
    static BusClient *create(QQmlEngine *qml, QJSEngine *js);
    static BusClient *of(QObject *obj);

    bool connected() const { return m_connected; }
    QString socketPath() const;
    QString clientName() const;

    // data is shared by every subscriber, never mutate it
    Q_INVOKABLE int subscribe(const QString &topic, const QJSValue &fn);
    Q_INVOKABLE void unsubscribe(int token);

    Q_INVOKABLE int call(const QString &method, const QJSValue &args = QJSValue(), const QJSValue &cb = QJSValue());
    Q_INVOKABLE int run(const QString &line, const QJSValue &cb = QJSValue(), const QJSValue &stdinText = QJSValue());
    Q_INVOKABLE int ipc(const QString &target, const QString &fn, const QJSValue &args = QJSValue(),
                        const QJSValue &cb = QJSValue(), const QJSValue &client = QJSValue());
    Q_INVOKABLE int ipcGet(const QString &target, const QString &prop, const QJSValue &cb = QJSValue(),
                           const QJSValue &client = QJSValue());
    Q_INVOKABLE bool cancel(int id);

    Q_INVOKABLE void serve(const QString &target, const QJSValue &handler);
    Q_INVOKABLE void unserve(const QString &target, const QJSValue &handler);
    Q_INVOKABLE void serveAs(const QJSValue &handler, const QString &target);

    // ShellSettings seeds from shell.json while the daemon is away, a daemon frame always wins
    Q_INVOKABLE void seed(const QString &topic, const QJSValue &data);

    void listen(const QString &topic, BusListener *listener);
    void unlisten(const QString &topic, BusListener *listener);
    bool frame(const QString &topic, QJsonValue *data) const;

    // for tests
    int queued() const { return int(m_queue.size()); }
    bool isOpen() const { return m_open; }

signals:
    void connectedChanged();

private:
    enum Kind { Call, Run, Ipc };
    struct Request {
        int id;
        Kind kind;
        QJsonObject msg;
        QJSValue cb;
    };
    struct Pending {
        Kind kind;
        QJSValue cb;
    };
    struct Topic {
        QMap<int, QJSValue> subs;
        QList<BusListener *> listeners;
        bool has = false;
        bool fromDaemon = false;
        qint64 rev = 0;
        QJsonValue data;
        QJSValue js;
        bool jsValid = false;
    };

    void ensure();
    void post(std::function<void(BusWorker *)> fn);
    int request(Kind kind, QJsonObject msg, const QJSValue &cb);
    void sendNow(const Request &req);
    void finish(Kind kind, const QJSValue &cb, bool ok, const QJSValue &result, const QString &error);
    void dropTopicIfIdle(const QString &name);
    QJSValue jsData(Topic &t);
    void notify(const QString &name, const QJSValue &data, const QJSValue &info,
                const QJsonValue &json, const QList<QStringList> *paths, bool event);

    void receive(const QList<BusInbound> &batch);
    void onOpened();
    void onClosed();
    void onState(const BusInbound &in);
    void onEvent(const BusInbound &in);
    void onReply(const QJsonObject &msg);
    void onCmd(const QJsonObject &msg);
    void setConnected(bool v);

    QJSEngine *m_engine;
    QThread *m_thread = nullptr;
    BusWorker *m_worker = nullptr;
    mutable QString m_clientName;
    bool m_open = false;
    bool m_connected = false;
    int m_nextId = 0;
    int m_nextToken = 0;
    QHash<QString, Topic> m_topics;
    QHash<int, QString> m_tokenTopic;
    // target -> handlers, newest last
    QHash<QString, QList<QJSValue>> m_targets;
    QList<Request> m_queue;
    QHash<int, Pending> m_inflight;
};
