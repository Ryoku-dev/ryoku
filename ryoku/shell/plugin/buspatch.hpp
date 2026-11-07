#pragma once

#include <QJSValue>
#include <QJsonArray>
#include <QJsonValue>
#include <QList>
#include <QStringList>

class QJSEngine;

// Applies a daemon delta frame. Ops are {"p": [...keys], "v": value} (set) or
// {"p": [...keys], "x": true} (delete); arrays arrive whole at their path. A
// set through a missing or non-object step makes it an object; a delete
// through one does nothing. Neither form mutates its input: the objects along
// each patched path are copied and untouched subtrees are shared.
namespace buspatch {

// The worker's copy of a topic frame.
QJsonValue apply(const QJsonValue &root, const QJsonArray &ops);

// The JS copy subscribers hold. Untouched subtrees keep their identity, so a
// binding on an unrelated key stays asleep.
QJSValue apply(QJSEngine *engine, const QJSValue &root, const QJsonArray &ops);

// The keys each op changed, in order.
QList<QStringList> paths(const QJsonArray &ops);

}
