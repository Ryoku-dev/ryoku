#pragma once

#include <QString>
#include <QVariant>

#include <cmath>

// An unset key falls back to the schema default, else the caller's fallback.
class ParamSource
{
public:
    virtual ~ParamSource() = default;

    virtual QVariant value(const QString &key) const = 0;
    virtual bool smallScreen() const = 0;

    double num(const QString &key, double fallback) const
    {
        const QVariant v = value(key);
        bool ok = false;
        const double d = v.toDouble(&ok);
        return ok && std::isfinite(d) ? d : fallback;
    }
    bool flag(const QString &key, bool fallback) const
    {
        const QVariant v = value(key);
        return v.isValid() && !v.isNull() ? v.toBool() : fallback;
    }
    QString text(const QString &key, const QString &fallback) const
    {
        const QVariant v = value(key);
        return v.isValid() && !v.isNull() ? v.toString() : fallback;
    }
};

// Settle times in milliseconds, scaled by the global motion speed.
struct MotionProfile {
    enum Tier { Fast, Standard, Slow };

    double fastMs = 180;
    double standardMs = 250;
    double slowMs = 450;
    double scale = 1.0;        // global multiplier (0 = instant)
    bool reduced = false;      // reduced motion: snap instead of animate

    double ms(Tier tier) const
    {
        const double base = tier == Fast ? fastMs : tier == Standard ? standardMs : slowMs;
        return reduced ? 1.0 : base * scale;
    }
    double tau(Tier tier) const { return ms(tier) / 4000.0; }
};
