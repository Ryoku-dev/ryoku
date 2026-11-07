#include "fakedaemon.hpp"

#include <QFile>
#include <QJsonDocument>
#include <QLocalSocket>

FakeDaemon::FakeDaemon(const QString &dir)
    : m_path(dir + QStringLiteral("/ryoku-shell.sock"))
{
    connect(&m_server, &QLocalServer::newConnection, this, &FakeDaemon::accept);
}

bool FakeDaemon::listen()
{
    QFile::remove(m_path);
    return m_server.listen(m_path);
}

void FakeDaemon::close()
{
    drop();
    m_server.close();
    QFile::remove(m_path);
}

bool FakeDaemon::connected() const
{
    return m_conn && m_conn->state() == QLocalSocket::ConnectedState;
}

void FakeDaemon::accept()
{
    while (QLocalSocket *s = m_server.nextPendingConnection()) {
        if (m_conn)
            m_conn->deleteLater();
        m_conn = s;
        m_buf.clear();
        m_lines.clear();
        ++m_connections;
        connect(s, &QLocalSocket::readyRead, this, [this, s] {
            if (s != m_conn)
                return;
            m_buf += s->readAll();
            qsizetype nl;
            while ((nl = m_buf.indexOf('\n')) >= 0) {
                m_lines.append(QString::fromUtf8(m_buf.left(nl)));
                m_buf.remove(0, nl + 1);
            }
        });
    }
}

QList<QJsonObject> FakeDaemon::ops(const QString &op) const
{
    QList<QJsonObject> out;
    for (const QString &line : m_lines) {
        if (line.startsWith(QLatin1String("bus "))) {
            if (op == QLatin1String("bus"))
                out.append(QJsonDocument::fromJson(line.mid(4).toUtf8()).object());
            continue;
        }
        const QJsonObject o = QJsonDocument::fromJson(line.toUtf8()).object();
        if (o.value(QLatin1String("op")).toString() == op)
            out.append(o);
    }
    return out;
}

void FakeDaemon::send(const QJsonObject &msg)
{
    sendRaw(QJsonDocument(msg).toJson(QJsonDocument::Compact) + '\n');
}

void FakeDaemon::sendRaw(const QByteArray &bytes)
{
    if (!m_conn)
        return;
    m_conn->write(bytes);
    m_conn->flush();
}

void FakeDaemon::drop()
{
    if (m_conn) {
        m_conn->abort();
        m_conn->deleteLater();
    }
    m_conn = nullptr;
}
