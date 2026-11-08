#pragma once

#include <QJSValue>
#include <QJsonArray>
#include <QJsonValue>
#include <QList>
#include <QStringList>

class QJSEngine;

// never mutates its input, only the objects along each patched path are copied
namespace buspatch {

QJsonValue apply(const QJsonValue &root, const QJsonArray &ops);

// untouched subtrees keep their identity so a binding on an unrelated key stays asleep
QJSValue apply(QJSEngine *engine, const QJSValue &root, const QJsonArray &ops);

QList<QStringList> paths(const QJsonArray &ops);

}
