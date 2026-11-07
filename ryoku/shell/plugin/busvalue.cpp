#include "busvalue.hpp"

#include "buspath.hpp"

#include <QQmlEngine>

BusValue::BusValue(QObject *parent)
    : QObject(parent)
{
}

BusValue::~BusValue()
{
    detach();
}

void BusValue::classBegin()
{
    m_begun = true;
    m_client = BusClient::of(this);
    attach();
}

void BusValue::setTopic(const QString &topic)
{
    if (m_topic == topic)
        return;
    m_topic = topic;
    emit topicChanged();
    attach();
}

void BusValue::setPath(const QJSValue &path)
{
    if (path.strictlyEquals(m_path))
        return;
    m_path = path;
    m_keys = buspath::split(path);
    emit pathChanged();
    refresh();
}

void BusValue::setFallback(const QJSValue &fallback)
{
    if (fallback.strictlyEquals(m_fallback))
        return;
    m_fallback = fallback;
    emit fallbackChanged();
    if (!m_present)
        emit valueChanged();
}

void BusValue::attach()
{
    if (!m_begun || m_listening == m_topic)
        return;
    detach();
    if (m_client && !m_topic.isEmpty()) {
        m_client->listen(m_topic, this);
        m_listening = m_topic;
    }
    refresh();
}

void BusValue::detach()
{
    if (m_client && !m_listening.isEmpty())
        m_client->unlisten(m_listening, this);
    m_listening.clear();
}

void BusValue::refresh()
{
    QJsonValue data;
    const bool has = m_client && !m_listening.isEmpty() && m_client->frame(m_listening, &data);
    take(data, has);
}

void BusValue::busFrame(const QJsonValue &data, const QList<QStringList> *paths)
{
    if (!buspath::touches(m_keys, paths))
        return;
    take(data, true);
}

void BusValue::take(const QJsonValue &data, bool has)
{
    bool found = false;
    const QJsonValue v = has ? buspath::at(data, m_keys, &found) : QJsonValue(QJsonValue::Undefined);
    if (found == m_present && v == m_json)
        return;
    m_present = found;
    m_json = v;
    QJSEngine *engine = qmlEngine(this);
    m_value = found && engine ? engine->toScriptValue(v) : QJSValue();
    emit valueChanged();
}
