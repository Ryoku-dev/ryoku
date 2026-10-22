#pragma once

#include <qmetaobject.h>
#include <qmetatype.h>
#include <qvariant.h>

namespace util {

inline QMetaEnum metaEnumFor(const QMetaType& type) {
    const auto* meta = type.metaObject();
    if (!meta)
        return {};

    auto name = QByteArray(type.name());
    if (const auto scope = name.lastIndexOf("::"); scope >= 0)
        name = name.mid(scope + 2);

    return meta->enumerator(meta->indexOfEnumerator(name.constData()));
}

inline bool isSupportedEnum(const QMetaType& type) {
    if (!type.flags().testFlag(QMetaType::IsEnumeration))
        return false;

    const auto metaEnum = metaEnumFor(type);
    return metaEnum.isValid() && !metaEnum.is64Bit();
}

// Returns the enumerator name for a value, or nullptr if there is no such enumerator
inline const char* enumKeyFor(const QMetaEnum& metaEnum, const QVariant& value) {
    return metaEnum.valueToKey(static_cast<quint64>(value.toLongLong()));
}

} // namespace util
