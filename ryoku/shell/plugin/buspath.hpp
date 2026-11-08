#pragma once

#include <QJSValue>
#include <QJsonArray>
#include <QJsonValue>
#include <QList>
#include <QStringList>
#include <QVariant>

// same rules as ryoku/ui/Singletons/buspath.js, keep the two in step
namespace buspath {

QStringList split(const QString &dotted);
QStringList split(const QJSValue &path);
QStringList split(const QVariant &path);
QStringList keys(const QJsonArray &p);

QJsonValue at(const QJsonValue &root, const QStringList &keys, bool *found = nullptr);

// a null changed list means everything may have changed
bool touches(const QStringList &path, const QList<QStringList> *changed);

}
