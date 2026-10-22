#include "common.hpp"

#include "node.hpp"

namespace caelestia::settings {

Q_LOGGING_CATEGORY(lcSettings, "caelestia.settings", QtInfoMsg)

WriteScope::WriteScope(Node* node, WriteOrigin origin)
    : m_root(node->rootNode()) {
    m_previous = m_root->m_writeOrigin;
    m_root->m_writeOrigin = origin;
}

WriteScope::~WriteScope() {
    m_root->m_writeOrigin = m_previous;
}

InternalRead::InternalRead(Node* node)
    : m_root(node->rootNode())
    , m_previous(m_root->m_internalRead) {
    m_root->m_internalRead = true;
}

InternalRead::~InternalRead() {
    m_root->m_internalRead = m_previous;
}

QString DiagnosticType::toString(Type t) {
    switch (t) {
    case UnknownOption:
        return QStringLiteral("UnknownOption");
    case GlobalOption:
        return QStringLiteral("GlobalOption");
    case TypeMismatch:
        return QStringLiteral("TypeMismatch");
    case InvalidValue:
        return QStringLiteral("InvalidValue");
    }
}

namespace {

QString expectedStr(ExpectedType expected) {
    switch (expected) {
    case ExpectedType::Bool:
        return QStringLiteral("a boolean");
    case ExpectedType::Int:
        return QStringLiteral("an integer");
    case ExpectedType::Real:
        return QStringLiteral("a number");
    case ExpectedType::String:
        return QStringLiteral("a string");
    case ExpectedType::Array:
        return QStringLiteral("an array");
    case ExpectedType::Object:
        return QStringLiteral("an object");
    }

    Q_UNREACHABLE_RETURN(QString());
}

QString receivedStr(const QJsonValue& value) {
    switch (value.type()) {
    case QJsonValue::Null:
        return QStringLiteral("null");
    case QJsonValue::Bool:
        return QStringLiteral("a boolean");
    case QJsonValue::Double:
        return QStringLiteral("a number");
    case QJsonValue::String:
        return QStringLiteral("a string");
    case QJsonValue::Array:
        return QStringLiteral("an array");
    case QJsonValue::Object:
        return QStringLiteral("an object");
    default:
        return QStringLiteral("nothing");
    }
}

} // namespace

Diagnostic Diagnostic::mismatch(ExpectedType expected, const QJsonValue& value, const QString& option) {
    return {
        .type = DiagnosticType::TypeMismatch,
        .option = option,
        .message = QStringLiteral("Expected %1, got %2").arg(expectedStr(expected), receivedStr(value)),
    };
}

Diagnostic Diagnostic::mismatch(const QList<ExpectedType>& expected, const QJsonValue& value, const QString& option) {
    QStringList args;
    args.reserve(expected.size() + 1);
    for (const auto type : expected)
        args << expectedStr(type);
    args << receivedStr(value);

    switch (expected.size()) {
    case 2:
        return {
            .type = DiagnosticType::TypeMismatch,
            .option = option,
            .message = QStringLiteral("Expected %1 or %2, got %3").arg(args[0], args[1], args[2]),
        };
    case 3:
        return {
            .type = DiagnosticType::TypeMismatch,
            .option = option,
            .message = QStringLiteral("Expected %1, %2 or %3, got %4").arg(args[0], args[1], args[2], args[3]),
        };
    case 4:
        return {
            .type = DiagnosticType::TypeMismatch,
            .option = option,
            .message = QStringLiteral("Expected one of: %1, %2, %3, %4; got %5")
                .arg(args[0], args[1], args[2], args[3], args[4]),
        };
    default:
        Q_UNREACHABLE_RETURN(Diagnostic{});
    }
}

} // namespace caelestia::settings
