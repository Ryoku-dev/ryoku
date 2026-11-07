#pragma once

#include <QJSValue>
#include <QJsonArray>
#include <QJsonValue>
#include <QList>
#include <QStringList>
#include <QVariant>

// Key paths into a topic frame, with the same rules as the QML side
// (ryoku/ui/Singletons/buspath.js): a path is "bar.style" or ["bar", "style"],
// and a delta frame names what it changed as key arrays.
namespace buspath {

QStringList split(const QString &dotted);
QStringList split(const QJSValue &path);
QStringList split(const QVariant &path);
// A patch op's "p": its keys as strings (a number key becomes its digits).
QStringList keys(const QJsonArray &p);

// The value at keys; found is false when a step is missing or not a
// container. An array step takes a decimal index.
QJsonValue at(const QJsonValue &root, const QStringList &keys, bool *found = nullptr);

// Whether a change at any of changed can move the value at path: the change is
// at the path, inside it, or replaces an ancestor. A null changed list means
// everything may have changed.
bool touches(const QStringList &path, const QList<QStringList> *changed);

}
