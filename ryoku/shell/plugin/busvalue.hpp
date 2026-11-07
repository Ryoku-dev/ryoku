#pragma once

#include "busclient.hpp"

#include <QJSValue>
#include <QJsonValue>
#include <QObject>
#include <QPointer>
#include <QQmlParserStatus>
#include <QStringList>
#include <QtQml/qqmlregistration.h>

// One value inside a topic frame, e.g. BusValue { topic: "settings"; path:
// "bar.style" }. value changes only when the value at path does: a patch that
// does not reach the path is skipped without a look, and a full frame that is
// equal there changes nothing. fallback stands in while the path is missing.
class BusValue : public QObject, public QQmlParserStatus, public BusListener
{
    Q_OBJECT
    QML_ELEMENT
    Q_INTERFACES(QQmlParserStatus)
    Q_PROPERTY(QString topic READ topic WRITE setTopic NOTIFY topicChanged)
    Q_PROPERTY(QJSValue path READ path WRITE setPath NOTIFY pathChanged)
    Q_PROPERTY(QJSValue fallback READ fallback WRITE setFallback NOTIFY fallbackChanged)
    Q_PROPERTY(QJSValue value READ value NOTIFY valueChanged)
    Q_PROPERTY(bool present READ present NOTIFY valueChanged)

public:
    explicit BusValue(QObject *parent = nullptr);
    ~BusValue() override;

    QString topic() const { return m_topic; }
    void setTopic(const QString &topic);
    QJSValue path() const { return m_path; }
    void setPath(const QJSValue &path);
    QJSValue fallback() const { return m_fallback; }
    void setFallback(const QJSValue &fallback);
    QJSValue value() const { return m_present ? m_value : m_fallback; }
    bool present() const { return m_present; }

    void classBegin() override;
    void componentComplete() override {}
    void busFrame(const QJsonValue &data, const QList<QStringList> *paths) override;

signals:
    void topicChanged();
    void pathChanged();
    void fallbackChanged();
    void valueChanged();

private:
    void attach();
    void detach();
    void refresh();
    void take(const QJsonValue &data, bool has);

    QPointer<BusClient> m_client;
    QString m_topic;
    QString m_listening;
    QJSValue m_path;
    QStringList m_keys;
    QJSValue m_fallback;
    QJsonValue m_json = QJsonValue(QJsonValue::Undefined);
    QJSValue m_value;
    bool m_present = false;
    bool m_begun = false;
};
