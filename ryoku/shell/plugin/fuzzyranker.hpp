#pragma once

#include <QJSValue>
#include <QObject>
#include <QString>
#include <QtQml/qqmlregistration.h>

class QJSEngine;
class QQmlEngine;

class FuzzyRanker : public QObject
{
    Q_OBJECT
    QML_NAMED_ELEMENT(FuzzyRanker)
    QML_SINGLETON

public:
    explicit FuzzyRanker(QJSEngine *engine, QObject *parent = nullptr);
    static FuzzyRanker *create(QQmlEngine *qml, QJSEngine *js);

    Q_INVOKABLE QJSValue rank(QJSValue entries, QString query, QJSValue usage = QJSValue()) const;
    Q_INVOKABLE int score(QJSValue entry, QString query) const;
    Q_INVOKABLE QJSValue highlight(QString name, QString query) const;
    Q_INVOKABLE QJSValue prefixRank(QJSValue entries, QString query, QString field) const;

private:
    QJSEngine *m_engine = nullptr;
};
