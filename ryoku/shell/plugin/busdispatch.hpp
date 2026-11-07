#pragma once

#include <QJSValue>
#include <QJsonObject>
#include <QString>

class QJSEngine;

// A daemon cmd run against a served handler, with qs ipc's rules: arguments
// arrive as strings and only "true"/"false" become booleans (typed QML
// parameters coerce the rest); the argument count must match the
// declaration; the result is the return value as text.
namespace busdispatch {

// What a cmd may reach on a handler: the functions its file declares, not the
// object machinery every QML object carries.
bool callable(const QJSValue &handler, const QString &name);
bool readable(const QJSValue &handler, const QString &name);
QString arityError(const QJSValue &want, int got);
QString resultText(const QJSValue &ret);

// The done message for cmd (without "op"/"id"); handler undefined means
// nobody serves the target.
QJsonObject run(QJSEngine *engine, const QJSValue &handler, const QJsonObject &cmd);

}
