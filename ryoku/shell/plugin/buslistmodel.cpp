#include "buslistmodel.hpp"

#include "buslistdiff.hpp"
#include "buspath.hpp"

#include <QJsonArray>
#include <QJsonDocument>
#include <QQmlEngine>
#include <QSet>

namespace {

constexpr int ModelDataRole = Qt::UserRole;
constexpr int FirstRole = Qt::UserRole + 1;

// keys compare like JS ===, 5 and "5" differ but 5 and 5.0 do not
QString keyText(const QJsonValue &v)
{
    switch (v.type()) {
    case QJsonValue::String:
        return QLatin1Char('s') + v.toString();
    case QJsonValue::Double: {
        const double d = v.toDouble();
        if (d >= -9e15 && d <= 9e15 && double(qint64(d)) == d)
            return QLatin1Char('n') + QString::number(qint64(d));
        return QLatin1Char('n') + QString::number(d, 'g', 17);
    }
    case QJsonValue::Bool:
        return v.toBool() ? QStringLiteral("b1") : QStringLiteral("b0");
    case QJsonValue::Null:
        return QStringLiteral("z");
    case QJsonValue::Object:
        return QLatin1Char('j') + QString::fromUtf8(QJsonDocument(v.toObject()).toJson(QJsonDocument::Compact));
    case QJsonValue::Array:
        return QLatin1Char('j') + QString::fromUtf8(QJsonDocument(v.toArray()).toJson(QJsonDocument::Compact));
    case QJsonValue::Undefined:
        break;
    }
    return {};
}

QString keyText(const QVariant &v)
{
    if (!v.isValid())
        return {};
    if (v.metaType().flags() & QMetaType::PointerToQObject)
        return QLatin1Char('o') + QString::number(quintptr(v.value<QObject *>()), 16);
    if (v.metaType() == QMetaType::fromType<QJSValue>())
        return keyText(v.value<QJSValue>().toVariant());
    return keyText(QJsonValue::fromVariant(v));
}

bool isQObject(const QVariant &v)
{
    return v.metaType().flags() & QMetaType::PointerToQObject;
}

// a suffix no key text can contain tells duplicates apart by order
QString occurrence(QHash<QString, int> &seen, const QString &base)
{
    const int n = seen[base]++;
    return n == 0 ? base : base + QChar(0x1f) + QString::number(n);
}

}

BusListModel::BusListModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

BusListModel::~BusListModel()
{
    detach();
}

void BusListModel::classBegin()
{
    m_begun = true;
    m_client = BusClient::of(this);
}

void BusListModel::componentComplete()
{
    m_complete = true;
    attach();
    reload();
}

void BusListModel::setTopic(const QString &topic)
{
    if (m_topic == topic)
        return;
    m_topic = topic;
    emit topicChanged();
    if (!m_complete)
        return;
    attach();
    reload();
}

void BusListModel::setPath(const QString &path)
{
    if (m_path == path)
        return;
    m_path = path;
    m_pathKeys = buspath::split(path);
    emit pathChanged();
    reload();
}

void BusListModel::setKey(const QString &key)
{
    if (m_key == key)
        return;
    m_key = key;
    emit keyChanged();
    reload();
}

void BusListModel::setRoles(const QStringList &roles)
{
    if (m_explicitRoles == roles)
        return;
    m_explicitRoles = roles;
    emit rolesChanged();
    if (!m_complete || m_rows.isEmpty()) {
        m_roles = roles;
        return;
    }
    // views read role names once so new names need a reset
    beginResetModel();
    m_roles = roles;
    endResetModel();
    reload();
}

void BusListModel::setFilterRole(const QString &role)
{
    if (m_filterRole == role)
        return;
    m_filterRole = role;
    emit filterRoleChanged();
    reload();
}

void BusListModel::setFilterValue(const QVariant &value)
{
    if (m_filterValue == value && m_filterValue.metaType() == value.metaType())
        return;
    m_filterValue = value;
    emit filterValueChanged();
    reload();
}

void BusListModel::setRows(const QJSValue &rows)
{
    if (rows.strictlyEquals(m_source) && !rows.isObject())
        return;
    m_source = rows;
    emit rowsChanged();
    if (m_topic.isEmpty())
        reload();
}

void BusListModel::attach()
{
    if (!m_begun || m_listening == m_topic)
        return;
    detach();
    if (m_client && !m_topic.isEmpty()) {
        m_client->listen(m_topic, this);
        m_listening = m_topic;
    }
}

void BusListModel::detach()
{
    if (m_client && !m_listening.isEmpty())
        m_client->unlisten(m_listening, this);
    m_listening.clear();
}

void BusListModel::reload()
{
    if (!m_complete)
        return;
    bool keyed = true;
    QList<Row> next;
    if (!m_topic.isEmpty()) {
        QJsonValue frame;
        if (m_client && !m_listening.isEmpty() && m_client->frame(m_listening, &frame))
            next = fromJson(frame, &keyed);
    } else {
        next = fromRows(&keyed);
    }
    apply(std::move(next), keyed);
}

void BusListModel::busFrame(const QJsonValue &data, const QList<QStringList> *paths)
{
    if (!m_complete || !buspath::touches(m_pathKeys, paths))
        return;
    bool keyed = true;
    QList<Row> next = fromJson(data, &keyed);
    apply(std::move(next), keyed);
}

bool BusListModel::matches(const QJsonValue &field) const
{
    if (!m_filterValue.isValid() || m_filterValue.isNull())
        return field.isUndefined() || field.isNull();
    return field == QJsonValue::fromVariant(m_filterValue);
}

bool BusListModel::matches(const QVariant &field) const
{
    if (!m_filterValue.isValid() || m_filterValue.isNull())
        return !field.isValid() || field.isNull();
    if (isQObject(field) || isQObject(m_filterValue))
        return isQObject(field) && isQObject(m_filterValue)
            && field.value<QObject *>() == m_filterValue.value<QObject *>();
    return QJsonValue::fromVariant(field) == QJsonValue::fromVariant(m_filterValue);
}

QList<BusListModel::Row> BusListModel::fromJson(const QJsonValue &frame, bool *keyed) const
{
    const QJsonArray items = buspath::at(frame, m_pathKeys).toArray();
    QList<Row> out;
    out.reserve(items.size());
    QHash<QString, int> seen;
    seen.reserve(items.size());
    for (const QJsonValue &item : items) {
        Row row;
        if (!item.isObject()) {
            *keyed = false;
            row.id = QChar(0x1e) + QString::number(out.size());
            out.append(std::move(row));
            continue;
        }
        row.json = item.toObject();
        if (!m_filterRole.isEmpty() && !matches(row.json.value(m_filterRole)))
            continue;
        const QString base = keyText(row.json.value(m_key));
        if (base.isEmpty()) {
            *keyed = false;
            row.id = QChar(0x1e) + QString::number(out.size());
        } else {
            row.id = occurrence(seen, base);
        }
        out.append(std::move(row));
    }
    return out;
}

QList<BusListModel::Row> BusListModel::fromRows(bool *keyed) const
{
    QList<Row> out;
    if (!m_source.isObject())
        return out;
    const int n = m_source.property(QStringLiteral("length")).toInt();
    out.reserve(n);
    // with roles given only those fields are read, not the whole object
    QStringList fields;
    if (!m_explicitRoles.isEmpty()) {
        fields = m_explicitRoles;
        if (!fields.contains(m_key))
            fields.append(m_key);
        if (!m_filterRole.isEmpty() && !fields.contains(m_filterRole))
            fields.append(m_filterRole);
    }
    QHash<QString, int> seen;
    for (int i = 0; i < n; ++i) {
        Row row;
        row.isLocal = true;
        row.js = m_source.property(quint32(i));
        if (!row.js.isObject() || row.js.isArray() || row.js.isQObject() || row.js.isCallable()) {
            *keyed = false;
            row.id = QChar(0x1e) + QString::number(out.size());
            out.append(std::move(row));
            continue;
        }
        if (fields.isEmpty()) {
            row.local = row.js.toVariant().toMap();
        } else {
            for (const QString &f : std::as_const(fields)) {
                const QJSValue v = row.js.property(f);
                if (!v.isUndefined())
                    row.local.insert(f, v.toVariant());
            }
        }
        if (!m_filterRole.isEmpty() && !matches(row.local.value(m_filterRole)))
            continue;
        const QString base = keyText(row.local.value(m_key));
        if (base.isEmpty()) {
            *keyed = false;
            row.id = QChar(0x1e) + QString::number(out.size());
        } else {
            row.id = occurrence(seen, base);
        }
        out.append(std::move(row));
    }
    return out;
}

void BusListModel::deriveRoles(const QList<Row> &rows)
{
    if (!m_roles.isEmpty())
        return;
    if (!m_explicitRoles.isEmpty()) {
        m_roles = m_explicitRoles;
        return;
    }
    QSet<QString> seen;
    for (const Row &row : rows) {
        const QStringList keys = row.isLocal ? row.local.keys() : row.json.keys();
        for (const QString &k : keys) {
            if (!seen.contains(k)) {
                seen.insert(k);
                m_roles.append(k);
            }
        }
    }
}

bool BusListModel::fieldEqual(const Row &a, const Row &b, const QString &name) const
{
    if (!a.isLocal && !b.isLocal)
        return a.json.value(name) == b.json.value(name);
    if (a.isLocal && b.isLocal) {
        const QVariant x = a.local.value(name);
        const QVariant y = b.local.value(name);
        if (isQObject(x) || isQObject(y))
            return isQObject(x) && isQObject(y) && x.value<QObject *>() == y.value<QObject *>();
        return x == y;
    }
    return false;
}

QList<int> BusListModel::changedRoles(const Row &a, const Row &b) const
{
    QList<int> out;
    // one deep compare settles the common unchanged row
    if (a.isLocal == b.isLocal && (a.isLocal ? a.local == b.local : a.json == b.json))
        return out;
    for (int i = 0; i < m_roles.size(); ++i)
        if (!fieldEqual(a, b, m_roles.at(i)))
            out.append(FirstRole + i);
    if (!out.isEmpty() || m_explicitRoles.isEmpty())
        out.append(ModelDataRole);
    return out;
}

void BusListModel::apply(QList<Row> next, bool keyed)
{
    const int before = count();
    m_indexValid = false;
    if (!keyed) {
        beginResetModel();
        deriveRoles(next);
        m_rows = std::move(next);
        endResetModel();
        if (count() != before)
            emit countChanged();
        return;
    }
    deriveRoles(next);

    QStringList oldIds;
    oldIds.reserve(m_rows.size());
    for (const Row &r : std::as_const(m_rows))
        oldIds.append(r.id);
    QStringList newIds;
    newIds.reserve(next.size());
    for (const Row &r : std::as_const(next))
        newIds.append(r.id);

    const QList<buslistdiff::Op> ops = buslistdiff::diff(oldIds, newIds);
    for (const buslistdiff::Op &op : ops) {
        switch (op.kind) {
        case buslistdiff::Op::Remove:
            beginRemoveRows({}, op.row, op.row + op.count - 1);
            m_rows.remove(op.row, op.count);
            endRemoveRows();
            break;
        case buslistdiff::Op::Insert:
            beginInsertRows({}, op.row, op.row + op.count - 1);
            for (int m = 0; m < op.count; ++m)
                m_rows.insert(op.row + m, next.at(op.source + m));
            endInsertRows();
            break;
        case buslistdiff::Op::Move: {
            // Qt wants the row it lands before, counted before the move
            const int dest = op.to > op.row ? op.to + 1 : op.to;
            beginMoveRows({}, op.row, op.row, {}, dest);
            m_rows.move(op.row, op.to);
            endMoveRows();
            break;
        }
        }
    }

    for (int i = 0; i < m_rows.size(); ++i) {
        const QList<int> changed = changedRoles(m_rows.at(i), next.at(i));
        m_rows[i] = std::move(next[i]);
        if (!changed.isEmpty()) {
            const QModelIndex at = index(i);
            emit dataChanged(at, at, changed);
        }
    }
    if (count() != before)
        emit countChanged();
}

int BusListModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : int(m_rows.size());
}

QVariant BusListModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() >= m_rows.size())
        return {};
    const Row &row = m_rows.at(index.row());
    if (role == ModelDataRole) {
        if (m_explicitRoles.isEmpty())
            return row.isLocal ? QVariant::fromValue(row.js) : QVariant::fromValue(row.json);
        if (row.isLocal) {
            QVariantMap out;
            for (const QString &k : std::as_const(m_roles))
                if (row.local.contains(k))
                    out.insert(k, row.local.value(k));
            return out;
        }
        QJsonObject out;
        for (const QString &k : std::as_const(m_roles)) {
            const auto it = row.json.constFind(k);
            if (it != row.json.constEnd())
                out.insert(k, it.value());
        }
        return QVariant::fromValue(out);
    }
    const int i = role - FirstRole;
    if (i < 0 || i >= m_roles.size())
        return {};
    const QString &name = m_roles.at(i);
    if (row.isLocal)
        return row.local.value(name);
    const QJsonValue v = row.json.value(name);
    if (v.isUndefined())
        return {};
    if (v.isObject() || v.isArray())
        return QVariant::fromValue(v);
    return v.toVariant();
}

QHash<int, QByteArray> BusListModel::roleNames() const
{
    QHash<int, QByteArray> out;
    out.insert(ModelDataRole, QByteArrayLiteral("modelData"));
    for (int i = 0; i < m_roles.size(); ++i)
        out.insert(FirstRole + i, m_roles.at(i).toUtf8());
    return out;
}

QJSValue BusListModel::get(int row) const
{
    if (row < 0 || row >= m_rows.size())
        return {};
    const Row &r = m_rows.at(row);
    if (r.isLocal)
        return r.js;
    QJSEngine *engine = qmlEngine(this);
    return engine ? engine->toScriptValue(r.json) : QJSValue();
}

int BusListModel::indexOf(const QVariant &key) const
{
    if (!m_indexValid) {
        m_index.clear();
        m_index.reserve(m_rows.size());
        for (int i = 0; i < m_rows.size(); ++i)
            m_index.insert(m_rows.at(i).id, i);
        m_indexValid = true;
    }
    const QString id = keyText(key);
    return id.isEmpty() ? -1 : m_index.value(id, -1);
}
