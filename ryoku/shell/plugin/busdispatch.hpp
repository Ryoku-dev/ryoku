#pragma once

#include <QJSValue>
#include <QJsonObject>
#include <QString>

class QJSEngine;

// same rules as qs ipc so a served handler answers like an IpcHandler
namespace busdispatch {

bool callable(const QJSValue &handler, const QString &name);
bool readable(const QJSValue &handler, const QString &name);
QString arityError(const QJSValue &want, int got);
QString resultText(const QJSValue &ret);

QJsonObject run(QJSEngine *engine, const QJSValue &handler, const QJsonObject &cmd);

}
