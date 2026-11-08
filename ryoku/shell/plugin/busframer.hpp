#pragma once

#include <QByteArray>
#include <QList>

class BusFramer
{
public:
    void append(const QByteArray &chunk, QList<QByteArray> &lines);
    void clear();
    qsizetype pending() const { return m_buf.size(); }

private:
    QByteArray m_buf;
    qsizetype m_scan = 0;
};
