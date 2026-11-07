#pragma once

#include <QByteArray>
#include <QList>

// Splits the daemon socket's byte stream into wire lines (one JSON object per
// line, never a raw newline inside one). Empty lines are dropped. A frame can
// arrive in many reads; the scan resumes where the last one stopped, so a large
// frame costs one pass however it is chunked.
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
