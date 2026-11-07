#include "buspath.hpp"

#include <QJsonObject>

namespace buspath {

QStringList split(const QString &dotted)
{
    return dotted.isEmpty() ? QStringList() : dotted.split(QLatin1Char('.'));
}

QStringList split(const QJSValue &path)
{
    if (path.isUndefined() || path.isNull())
        return {};
    // A key list that crossed a QML var arrives as a sequence, which is not
    // an Array, so anything with a length that is not a string is a key list.
    if (!path.isString() && path.isObject() && path.property(QStringLiteral("length")).isNumber()) {
        const int n = path.property(QStringLiteral("length")).toInt();
        QStringList out;
        out.reserve(n);
        for (int i = 0; i < n; ++i)
            out.append(path.property(quint32(i)).toString());
        return out;
    }
    return split(path.toString());
}

QStringList split(const QVariant &path)
{
    if (path.metaType() == QMetaType::fromType<QJSValue>())
        return split(path.value<QJSValue>());
    if (path.metaType() == QMetaType::fromType<QStringList>())
        return path.toStringList();
    if (path.metaType() == QMetaType::fromType<QVariantList>()) {
        QStringList out;
        for (const QVariant &k : path.toList())
            out.append(k.toString());
        return out;
    }
    return path.isValid() ? split(path.toString()) : QStringList();
}

QStringList keys(const QJsonArray &p)
{
    QStringList out;
    out.reserve(p.size());
    for (const QJsonValue &k : p)
        out.append(k.isString() ? k.toString() : k.toVariant().toString());
    return out;
}

QJsonValue at(const QJsonValue &root, const QStringList &keys, bool *found)
{
    QJsonValue cur = root;
    for (const QString &k : keys) {
        if (cur.isObject()) {
            const QJsonObject o = cur.toObject();
            const auto it = o.constFind(k);
            if (it == o.constEnd()) {
                if (found)
                    *found = false;
                return QJsonValue(QJsonValue::Undefined);
            }
            cur = it.value();
        } else if (cur.isArray()) {
            bool ok = false;
            const qsizetype i = k.toLongLong(&ok);
            const QJsonArray a = cur.toArray();
            if (!ok || i < 0 || i >= a.size()) {
                if (found)
                    *found = false;
                return QJsonValue(QJsonValue::Undefined);
            }
            cur = a.at(i);
        } else {
            if (found)
                *found = false;
            return QJsonValue(QJsonValue::Undefined);
        }
    }
    if (found)
        *found = true;
    return cur;
}

bool touches(const QStringList &path, const QList<QStringList> *changed)
{
    if (!changed)
        return true;
    for (const QStringList &c : *changed) {
        const qsizetype n = qMin(c.size(), path.size());
        qsizetype j = 0;
        while (j < n && c.at(j) == path.at(j))
            ++j;
        if (j == n)
            return true;
    }
    return false;
}

}
