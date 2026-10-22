#pragma once

#include <qjsonvalue.h>
#include <qlist.h>
#include <qloggingcategory.h>
#include <qobject.h>
#include <qqmlintegration.h>

namespace caelestia::settings {

Q_DECLARE_LOGGING_CATEGORY(lcSettings)

class Node;

// The JSON shape an option is decoded from, used to build mismatch diagnostics
enum class ExpectedType {
    Bool = 0,
    Int,
    Real,
    String,
    Array,
    Object,
};

enum class WriteOrigin {
    Init,
    File,
    FileReset,
    Layer,
    Qml,
    QmlReset,
};

class WriteScope {
public:
    explicit WriteScope(Node* node, WriteOrigin origin);
    ~WriteScope();

private:
    Node* m_root;
    WriteOrigin m_previous;

    Q_DISABLE_COPY_MOVE(WriteScope)
};

class InternalRead {
public:
    explicit InternalRead(Node* node);
    ~InternalRead();

private:
    Node* const m_root;
    const bool m_previous;

    Q_DISABLE_COPY_MOVE(InternalRead)
};

class DiagnosticType : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

public:
    enum Type {
        UnknownOption = 0,
        GlobalOption,
        TypeMismatch,
        InvalidValue,
    };
    Q_ENUM(Type)

    Q_INVOKABLE QString toString(Type t);
};

struct Diagnostic {
    Q_GADGET
    QML_VALUE_TYPE(diagnostic)

    Q_PROPERTY(DiagnosticType::Type type MEMBER type)
    Q_PROPERTY(QString option MEMBER option)
    Q_PROPERTY(QString message MEMBER message)

public:
    DiagnosticType::Type type = DiagnosticType::UnknownOption;
    QString option;
    QString message;

    static Diagnostic mismatch(ExpectedType expected, const QJsonValue& value, const QString& option = {});
    static Diagnostic mismatch(
        const QList<ExpectedType>& expected, const QJsonValue& value, const QString& option = {});

    bool operator==(const Diagnostic& other) const = default;
};

} // namespace caelestia::settings
