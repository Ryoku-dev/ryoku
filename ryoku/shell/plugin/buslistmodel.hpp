#pragma once

#include "busclient.hpp"

#include <QAbstractListModel>
#include <QHash>
#include <QJSValue>
#include <QJsonObject>
#include <QPointer>
#include <QQmlParserStatus>
#include <QStringList>
#include <QVariant>
#include <QVariantMap>
#include <QtQml/qqmlregistration.h>

// A list model over one array inside a topic frame, e.g. BusListModel {
// topic: "wm"; path: "windows"; key: "id" }, or over a JS array given as rows
// while topic is "". Each frame is diffed by key against the rows the model
// holds, so a view sees inserts, removes, moves and per-role dataChanged, never
// a reset; only a row without the key (or one that is not an object) resets.
// Duplicate keys are told apart by their order. Roles are the given roles, or
// the union of the keys of the first non-empty frame, plus modelData (the row,
// or with roles given, just those roles); with roles given only they are
// compared.
class BusListModel : public QAbstractListModel, public QQmlParserStatus, public BusListener
{
    Q_OBJECT
    QML_ELEMENT
    Q_INTERFACES(QQmlParserStatus)
    Q_PROPERTY(QString topic READ topic WRITE setTopic NOTIFY topicChanged)
    Q_PROPERTY(QString path READ path WRITE setPath NOTIFY pathChanged)
    Q_PROPERTY(QString key READ key WRITE setKey NOTIFY keyChanged)
    Q_PROPERTY(QStringList roles READ roles WRITE setRoles NOTIFY rolesChanged)
    Q_PROPERTY(QString filterRole READ filterRole WRITE setFilterRole NOTIFY filterRoleChanged)
    Q_PROPERTY(QVariant filterValue READ filterValue WRITE setFilterValue NOTIFY filterValueChanged)
    Q_PROPERTY(QJSValue rows READ rows WRITE setRows NOTIFY rowsChanged)
    Q_PROPERTY(int count READ count NOTIFY countChanged)

public:
    explicit BusListModel(QObject *parent = nullptr);
    ~BusListModel() override;

    QString topic() const { return m_topic; }
    void setTopic(const QString &topic);
    QString path() const { return m_path; }
    void setPath(const QString &path);
    QString key() const { return m_key; }
    void setKey(const QString &key);
    QStringList roles() const { return m_explicitRoles; }
    void setRoles(const QStringList &roles);
    QString filterRole() const { return m_filterRole; }
    void setFilterRole(const QString &role);
    QVariant filterValue() const { return m_filterValue; }
    void setFilterValue(const QVariant &value);
    QJSValue rows() const { return m_source; }
    void setRows(const QJSValue &rows);
    int count() const { return int(m_rows.size()); }

    Q_INVOKABLE QJSValue get(int row) const;
    Q_INVOKABLE int indexOf(const QVariant &key) const;

    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    void classBegin() override;
    void componentComplete() override;
    void busFrame(const QJsonValue &data, const QList<QStringList> *paths) override;

signals:
    void topicChanged();
    void pathChanged();
    void keyChanged();
    void rolesChanged();
    void filterRoleChanged();
    void filterValueChanged();
    void rowsChanged();
    void countChanged();

private:
    struct Row {
        QString id;
        QJsonObject json;
        QVariantMap local;
        QJSValue js;
        bool isLocal = false;
    };

    void attach();
    void detach();
    void reload();
    // keyed is false when a row has no key or is not an object.
    QList<Row> fromJson(const QJsonValue &frame, bool *keyed) const;
    QList<Row> fromRows(bool *keyed) const;
    void deriveRoles(const QList<Row> &rows);
    void apply(QList<Row> next, bool keyed);
    QList<int> changedRoles(const Row &a, const Row &b) const;
    bool fieldEqual(const Row &a, const Row &b, const QString &name) const;
    bool matches(const QJsonValue &field) const;
    bool matches(const QVariant &field) const;

    QPointer<BusClient> m_client;
    QString m_topic;
    QString m_listening;
    QString m_path;
    QStringList m_pathKeys;
    QString m_key = QStringLiteral("id");
    QStringList m_explicitRoles;
    QStringList m_roles;
    QString m_filterRole;
    QVariant m_filterValue;
    QJSValue m_source;
    QList<Row> m_rows;
    mutable QHash<QString, int> m_index;
    mutable bool m_indexValid = false;
    bool m_begun = false;
    bool m_complete = false;
};
