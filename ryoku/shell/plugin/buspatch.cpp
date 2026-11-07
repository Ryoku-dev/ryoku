#include "buspatch.hpp"

#include "buspath.hpp"

#include <QJSEngine>
#include <QJSValueIterator>
#include <QJsonObject>

namespace buspatch {

namespace {

struct Op {
    QStringList keys;
    QJsonValue value;
    bool remove = false;
};

bool parse(const QJsonValue &raw, Op &op)
{
    const QJsonObject o = raw.toObject();
    const QJsonValue p = o.value(QLatin1String("p"));
    if (!p.isArray())
        return false;
    op.keys = buspath::keys(p.toArray());
    op.remove = o.value(QLatin1String("x")) == QJsonValue(true);
    op.value = o.value(QLatin1String("v"));
    return true;
}

// Returns false when a delete meets a missing step; obj is then unchanged.
bool applyAt(QJsonObject &obj, const Op &op, qsizetype i)
{
    const QString &k = op.keys.at(i);
    if (i == op.keys.size() - 1) {
        if (op.remove)
            obj.remove(k);
        else
            obj.insert(k, op.value);
        return true;
    }
    const QJsonValue child = obj.value(k);
    if (op.remove && !child.isObject())
        return false;
    QJsonObject next = child.toObject();
    if (!applyAt(next, op, i + 1))
        return false;
    obj.insert(k, next);
    return true;
}

bool isObject(const QJSValue &v)
{
    return v.isObject() && !v.isArray();
}

QJSValue shallow(QJSEngine *engine, const QJSValue &v)
{
    QJSValue out = engine->newObject();
    QJSValueIterator it(v);
    while (it.hasNext()) {
        it.next();
        out.setProperty(it.name(), it.value());
    }
    return out;
}

}

QJsonValue apply(const QJsonValue &root, const QJsonArray &ops)
{
    if (ops.isEmpty())
        return root;
    QJsonObject next = root.toObject();
    for (const QJsonValue &raw : ops) {
        Op op;
        if (!parse(raw, op))
            continue;
        if (op.keys.isEmpty()) {
            next = op.remove ? QJsonObject() : op.value.toObject();
            continue;
        }
        applyAt(next, op, 0);
    }
    return next;
}

QJSValue apply(QJSEngine *engine, const QJSValue &root, const QJsonArray &ops)
{
    if (ops.isEmpty())
        return root;
    QJSValue next = isObject(root) ? shallow(engine, root) : engine->newObject();
    // Objects this patch already copied: a second op under the same parent
    // writes into the copy instead of copying again.
    QList<QJSValue> fresh{next};
    const auto own = [&](QJSValue &parent, const QString &key) {
        const QJSValue child = parent.property(key);
        for (const QJSValue &f : std::as_const(fresh))
            if (f.strictlyEquals(child))
                return child;
        QJSValue copy = isObject(child) ? shallow(engine, child) : engine->newObject();
        parent.setProperty(key, copy);
        fresh.append(copy);
        return copy;
    };
    for (const QJsonValue &raw : ops) {
        Op op;
        if (!parse(raw, op))
            continue;
        if (op.keys.isEmpty()) {
            if (op.remove || !op.value.isObject())
                next = engine->newObject();
            else
                next = engine->toScriptValue(op.value);
            fresh = {next};
            continue;
        }
        QJSValue parent = next;
        bool missing = false;
        for (qsizetype j = 0; j < op.keys.size() - 1; ++j) {
            if (op.remove && !isObject(parent.property(op.keys.at(j)))) {
                missing = true;
                break;
            }
            parent = own(parent, op.keys.at(j));
        }
        if (missing)
            continue;
        const QString &last = op.keys.last();
        if (op.remove)
            parent.deleteProperty(last);
        else
            parent.setProperty(last, engine->toScriptValue(op.value));
    }
    return next;
}

QList<QStringList> paths(const QJsonArray &ops)
{
    QList<QStringList> out;
    out.reserve(ops.size());
    for (const QJsonValue &raw : ops) {
        const QJsonValue p = raw.toObject().value(QLatin1String("p"));
        if (p.isArray())
            out.append(buspath::keys(p.toArray()));
    }
    return out;
}

}
