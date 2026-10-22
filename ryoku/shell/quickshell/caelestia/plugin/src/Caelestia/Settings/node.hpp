#pragma once

#include <qjsonvalue.h>
#include <qobject.h>
#include <qset.h>
#include <qstringlist.h>
#include <qvariant.h>

#include "changebatcher.hpp"
#include "common.hpp"
#include "quarantine.hpp"
#include "schema.hpp"

namespace caelestia::settings {

class Node : public QObject {
    Q_OBJECT

    Q_PROPERTY(QStringList overrides READ overrides NOTIFY overridesChanged)

public:
    explicit Node(Node* fallback, QObject* parent = nullptr, bool globalOnly = false);

    [[nodiscard]] QString key() const;
    [[nodiscard]] QString path() const;
    [[nodiscard]] virtual QString pathFor(const QString& key) const;
    [[nodiscard]] Node* parentNode() const;
    [[nodiscard]] Node* rootNode() const;
    [[nodiscard]] Node* fallbackNode() const;
    void detachFallback();

    [[nodiscard]] Q_INVOKABLE bool isGlobalOnly() const;
    [[nodiscard]] Q_INVOKABLE bool isOverride(const QString& key) const;
    [[nodiscard]] QStringList overrides() const;
    [[nodiscard]] bool hasContent() const;

    [[nodiscard]] virtual const Schema& schema() const = 0;

    [[nodiscard]] virtual QVariant value(const QString& key) const;
    virtual bool setValue(const QString& key, const QVariant& value);
    virtual void resetToDefaults();

    [[nodiscard]] virtual QJsonValue toJson(bool sparse = true) const = 0;
    virtual bool syncJson(const QJsonValue& json, QList<Diagnostic>& diagnostics) = 0;
    [[nodiscard]] const Quarantine* quarantine() const;

signals:
    void optionChanged(const QString& key);
    void overridesChanged();

protected:
    // Null means empty, otherwise it has content
    std::unique_ptr<Quarantine> m_quarantine;
    const bool m_globalOnly;

    void warnGlobalRead(const QString& key) const;
    [[nodiscard]] bool rejectInvalidWrite(const QString& key, const QVariant& value) const;
    template <typename T> [[nodiscard]] bool rejectInvalidWrite(const QString& key, const T& value) const;
    // Returns true if the write should be skipped afterwards, overlays cannot write global options
    bool rejectGlobalWrite(const QString& key);
    static void warnGlobalSync(QList<Diagnostic>& diagnostics, const QString& path);
    bool rejectGlobalSync(QList<Diagnostic>& diagnostics) const;
    virtual bool recordWrite(const QString& key, bool changed);

    [[nodiscard]] bool removeQuarantined(const QString& key);
    [[nodiscard]] ChangeBatcher* batcher() const;

    [[nodiscard]] virtual QString keyOf(const Node* child) const;

    template <typename C, typename T>
    [[nodiscard]] T fallbackValue(T C::* member, std::type_identity_t<T> defaultValue) const;

private:
    QSet<QString> m_overrides;
    Node* const m_rootNode;
    Node* m_fallbackNode;

    WriteOrigin m_writeOrigin;
    bool m_internalRead;
    ChangeBatcher* const m_batcher;

    void onFallbackNotify(const QString& key);

    friend class WriteScope;
    friend class InternalRead;
};

template <typename T> bool Node::rejectInvalidWrite(const QString& key, const T& value) const {
    Q_UNUSED(key)
    Q_UNUSED(value)
    return false;
}

template <typename C, typename T> T Node::fallbackValue(T C::* member, std::type_identity_t<T> defaultValue) const {
    const auto* fallback = static_cast<const C*>(m_fallbackNode);
    return fallback ? fallback->*member : defaultValue;
}

} // namespace caelestia::settings
