#include "busframer.hpp"

void BusFramer::append(const QByteArray &chunk, QList<QByteArray> &lines)
{
    m_buf.append(chunk);
    qsizetype start = 0;
    for (;;) {
        const qsizetype nl = m_buf.indexOf('\n', m_scan);
        if (nl < 0)
            break;
        if (nl > start)
            lines.append(m_buf.mid(start, nl - start));
        start = nl + 1;
        m_scan = start;
    }
    if (start > 0)
        m_buf.remove(0, start);
    // what is left has been scanned and holds no newline
    m_scan = m_buf.size();
}

void BusFramer::clear()
{
    m_buf.clear();
    m_scan = 0;
}
