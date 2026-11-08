#include "busdispatch.hpp"

#include <QJSEngine>
#include <QJsonArray>

namespace busdispatch {

bool callable(const QJSValue &handler, const QString &name)
{
    static const QStringList denied{QStringLiteral("destroy"), QStringLiteral("toString"),
                                    QStringLiteral("deleteLater"), QStringLiteral("valueOf"),
                                    QStringLiteral("constructor")};
    return !name.isEmpty() && name.at(0) != QLatin1Char('_') && !denied.contains(name)
        && !name.endsWith(QLatin1String("Changed")) && handler.property(name).isCallable();
}

bool readable(const QJSValue &handler, const QString &name)
{
    return !name.isEmpty() && name.at(0) != QLatin1Char('_') && handler.hasProperty(name)
        && !handler.property(name).isCallable();
}

// qs ipc refuses a wrong argument count, a typed QML function would see undefined for a missing string
QString arityError(const QJSValue &want, int got)
{
    if (!want.isNumber())
        return {};
    const int n = want.toInt();
    if (n == got)
        return {};
    return QStringLiteral("%1 arguments provided (%2 required but %3 were provided.)")
        .arg(got < n ? QStringLiteral("Too few") : QStringLiteral("Too many"))
        .arg(n)
        .arg(got);
}

QString resultText(const QJSValue &ret)
{
    return ret.isUndefined() ? QString() : ret.toString();
}

QJsonObject run(QJSEngine *engine, const QJSValue &handler, const QJsonObject &cmd)
{
    QJsonObject done{{QStringLiteral("ok"), false}};
    const auto fail = [&](const QString &error) {
        done.insert(QStringLiteral("error"), error);
        return done;
    };
    if (handler.isUndefined())
        return fail(QStringLiteral("target not found"));
    const QJsonValue prop = cmd.value(QLatin1String("prop"));
    if (!prop.isUndefined() && !prop.isNull() && prop != QJsonValue(QString())) {
        const QString name = prop.toVariant().toString();
        if (!readable(handler, name))
            return fail(QStringLiteral("property not found"));
        const QJSValue v = handler.property(name);
        done.insert(QStringLiteral("ok"), true);
        done.insert(QStringLiteral("result"), resultText(v));
        return done;
    }
    const QString fn = cmd.value(QLatin1String("fn")).toVariant().toString();
    if (!callable(handler, fn))
        return fail(QStringLiteral("function not found"));
    const QJSValue f = handler.property(fn);
    QJSValueList args;
    for (const QJsonValue &a : cmd.value(QLatin1String("args")).toArray()) {
        if (a == QJsonValue(QStringLiteral("true")))
            args.append(QJSValue(true));
        else if (a == QJsonValue(QStringLiteral("false")))
            args.append(QJSValue(false));
        else
            args.append(engine->toScriptValue(a));
    }
    const QString bad = arityError(f.property(QStringLiteral("length")), int(args.size()));
    if (!bad.isEmpty())
        return fail(bad);
    const QJSValue ret = f.callWithInstance(handler, args);
    if (ret.isError())
        return fail(ret.toString());
    done.insert(QStringLiteral("ok"), true);
    done.insert(QStringLiteral("result"), resultText(ret));
    return done;
}

}
