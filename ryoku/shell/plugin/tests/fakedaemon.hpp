#pragma once

#include <QJsonObject>
#include <QList>
#include <QLocalServer>
#include <QObject>
#include <QPointer>
#include <QString>

class QLocalSocket;

// runs on the test thread, the QTRY_ macros spin its loop
class FakeDaemon : public QObject
{
    Q_OBJECT

public:
    explicit FakeDaemon(const QString &dir);
    bool listen();
    void close();

    QStringList lines() const { return m_lines; }
    // the bus line has op "bus"
    QList<QJsonObject> ops(const QString &op) const;
    int connections() const { return m_connections; }
    bool connected() const;

    void send(const QJsonObject &msg);
    void sendRaw(const QByteArray &bytes);
    void drop();

private:
    void accept();

    QString m_path;
    QLocalServer m_server;
    QPointer<QLocalSocket> m_conn;
    QByteArray m_buf;
    QStringList m_lines;
    int m_connections = 0;
};
