#pragma once

#include <qhash.h>
#include <qjsonvalue.h>
#include <qlist.h>
#include <qmetaobject.h>
#include <qvariant.h>

#include <optional>

#include "common.hpp"

namespace caelestia::settings {

struct DecodeResult {
    QVariant value;
    std::optional<Diagnostic> error;
    QList<qsizetype> indexPath;
};

class ValueCodec {
public:
    explicit ValueCodec(const QMetaType& type, ExpectedType expected);
    virtual ~ValueCodec() = default;

    static ValueCodec* codecFor(const QMetaType& type);

    static ValueCodec* unionFor(const QList<QMetaType>& types);

    [[nodiscard]] QMetaType type() const;
    [[nodiscard]] ExpectedType expected() const;

    [[nodiscard]] virtual QJsonValue encode(const QVariant& value) const = 0;
    [[nodiscard]] virtual DecodeResult decode(const QJsonValue& value) const = 0;

protected:
    const QMetaType m_type;
    const ExpectedType m_expected;

    Q_DISABLE_COPY_MOVE(ValueCodec)
};

#define CODEC(Type, Expected)                                                                                          \
    class Type##Codec : public ValueCodec {                                                                            \
    public:                                                                                                            \
        explicit Type##Codec(const QMetaType& type)                                                                    \
            : ValueCodec(type, ExpectedType::Expected) {}                                                              \
                                                                                                                       \
        [[nodiscard]] QJsonValue encode(const QVariant& value) const override;                                         \
        [[nodiscard]] DecodeResult decode(const QJsonValue& value) const override;                                     \
    };

CODEC(Bool, Bool)
CODEC(Int, Int)
CODEC(Real, Real)
CODEC(String, String)
CODEC(VariantList, Array)
CODEC(VariantMap, Object)

#undef CODEC

class EnumCodec : public ValueCodec {
public:
    explicit EnumCodec(const QMetaType& type, const QMetaEnum& metaEnum);

    [[nodiscard]] QJsonValue encode(const QVariant& value) const override;
    [[nodiscard]] DecodeResult decode(const QJsonValue& value) const override;

private:
    const QMetaEnum m_metaEnum;
};

template <typename Container> class ListCodec : public ValueCodec {
    using Value = typename Container::value_type;

public:
    explicit ListCodec(const QMetaType& type, const ValueCodec* elementCodec);

    [[nodiscard]] QJsonValue encode(const QVariant& value) const override;
    [[nodiscard]] DecodeResult decode(const QJsonValue& value) const override;

private:
    const ValueCodec* m_elementCodec;
};

// Decodes any one of several types, for options that accept more than one shape
class UnionCodec : public ValueCodec {
public:
    explicit UnionCodec(const QList<const ValueCodec*>& alternatives);

    [[nodiscard]] QJsonValue encode(const QVariant& value) const override;
    [[nodiscard]] DecodeResult decode(const QJsonValue& value) const override;

private:
    const QList<const ValueCodec*> m_alternatives;
    const QList<ExpectedType> m_expectedTypes;
    QHash<int, const ValueCodec*> m_byType;
};

} // namespace caelestia::settings
