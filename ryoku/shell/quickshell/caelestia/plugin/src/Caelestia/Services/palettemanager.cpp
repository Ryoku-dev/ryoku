// SPDX-License-Identifier: GPL-3.0-only
#include "palettemanager.hpp"

#include <qloggingcategory.h>

#include <algorithm>
#include <cmath>

Q_LOGGING_CATEGORY(lcPalette, "caelestia.services.palettemanager", QtInfoMsg)

namespace caelestia::services {

static const QStringList kPaletteKeys = {
    QStringLiteral("m3primary_paletteKeyColor"),
    QStringLiteral("m3secondary_paletteKeyColor"),
    QStringLiteral("m3tertiary_paletteKeyColor"),
    QStringLiteral("m3neutral_paletteKeyColor"),
    QStringLiteral("m3neutral_variant_paletteKeyColor"),
    QStringLiteral("m3background"),
    QStringLiteral("m3onBackground"),
    QStringLiteral("m3surface"),
    QStringLiteral("m3surfaceDim"),
    QStringLiteral("m3surfaceBright"),
    QStringLiteral("m3surfaceContainerLowest"),
    QStringLiteral("m3surfaceContainerLow"),
    QStringLiteral("m3surfaceContainer"),
    QStringLiteral("m3surfaceContainerHigh"),
    QStringLiteral("m3surfaceContainerHighest"),
    QStringLiteral("m3onSurface"),
    QStringLiteral("m3surfaceVariant"),
    QStringLiteral("m3onSurfaceVariant"),
    QStringLiteral("m3inverseSurface"),
    QStringLiteral("m3inverseOnSurface"),
    QStringLiteral("m3outline"),
    QStringLiteral("m3outlineVariant"),
    QStringLiteral("m3shadow"),
    QStringLiteral("m3scrim"),
    QStringLiteral("m3surfaceTint"),
    QStringLiteral("m3primary"),
    QStringLiteral("m3onPrimary"),
    QStringLiteral("m3primaryContainer"),
    QStringLiteral("m3onPrimaryContainer"),
    QStringLiteral("m3inversePrimary"),
    QStringLiteral("m3secondary"),
    QStringLiteral("m3onSecondary"),
    QStringLiteral("m3secondaryContainer"),
    QStringLiteral("m3onSecondaryContainer"),
    QStringLiteral("m3tertiary"),
    QStringLiteral("m3onTertiary"),
    QStringLiteral("m3tertiaryContainer"),
    QStringLiteral("m3onTertiaryContainer"),
    QStringLiteral("m3error"),
    QStringLiteral("m3onError"),
    QStringLiteral("m3errorContainer"),
    QStringLiteral("m3onErrorContainer"),
    QStringLiteral("m3success"),
    QStringLiteral("m3onSuccess"),
    QStringLiteral("m3successContainer"),
    QStringLiteral("m3onSuccessContainer"),
    QStringLiteral("m3primaryFixed"),
    QStringLiteral("m3primaryFixedDim"),
    QStringLiteral("m3onPrimaryFixed"),
    QStringLiteral("m3onPrimaryFixedVariant"),
    QStringLiteral("m3secondaryFixed"),
    QStringLiteral("m3secondaryFixedDim"),
    QStringLiteral("m3onSecondaryFixed"),
    QStringLiteral("m3onSecondaryFixedVariant"),
    QStringLiteral("m3tertiaryFixed"),
    QStringLiteral("m3tertiaryFixedDim"),
    QStringLiteral("m3onTertiaryFixed"),
    QStringLiteral("m3onTertiaryFixedVariant"),
};

static const QSet<QString> kLayer0Keys = {
    QStringLiteral("m3background"),
    QStringLiteral("m3surface"),
    QStringLiteral("m3surfaceDim"),
    QStringLiteral("m3surfaceBright"),
    QStringLiteral("m3surfaceVariant"),
    QStringLiteral("m3inverseSurface"),
};

PaletteManager::PaletteManager(QObject* parent)
    : QObject(parent) {}

QVariantMap PaletteManager::tPalette() const {
    return m_tPalette;
}

double PaletteManager::getLuminance(const QColor& c) const {
    const double r = c.redF();
    const double g = c.greenF();
    const double b = c.blueF();
    if (r == 0.0 && g == 0.0 && b == 0.0)
        return 0.0;
    return std::sqrt(0.299 * r * r + 0.587 * g * g + 0.114 * b * b);
}

QColor PaletteManager::applyLayer(const QColor& c, bool light, bool transpEnabled, double transpBase,
    double transpLayers, double wallLuminance, int layer) const {
    if (!transpEnabled)
        return c;

    if (layer == 0) {
        QColor result = c;
        result.setAlphaF(transpBase);
        return result;
    }

    const double luminance = getLuminance(c);
    if (luminance <= 0.0) {
        QColor result = c;
        result.setAlphaF(transpLayers);
        return result;
    }

    const double layerSign = (!light || layer == 1) ? 1.0 : (-static_cast<double>(layer) / 2.0);
    const double lightMul = light ? 0.2 : 0.3;
    const double wallFactor = light ? (layer == 1 ? 3.0 : 1.0) : 2.5;
    const double offset = layerSign * lightMul * (1.0 - transpBase) * (1.0 + wallLuminance * wallFactor);
    const double scale = (luminance + offset) / luminance;

    const double r = std::clamp(c.redF() * scale, 0.0, 1.0);
    const double g = std::clamp(c.greenF() * scale, 0.0, 1.0);
    const double b = std::clamp(c.blueF() * scale, 0.0, 1.0);

    return QColor::fromRgbF(r, g, b, transpLayers);
}

void PaletteManager::update(const QVariantMap& palette, bool light, bool transpEnabled, double transpBase,
    double transpLayers, double wallLuminance) {
    QVariantMap result;

    for (const auto& key : kPaletteKeys) {
        const auto raw = palette.value(key);
        QColor color;

        if (raw.typeId() == QMetaType::QColor) {
            color = raw.value<QColor>();
        } else {
            color = QColor(raw.toString());
        }

        if (!color.isValid()) {
            result.insert(key, raw);
            continue;
        }

        const int layer = kLayer0Keys.contains(key) ? 0 : 1;
        result.insert(key, applyLayer(color, light, transpEnabled, transpBase, transpLayers, wallLuminance, layer));
    }

    m_tPalette = result;
    emit tPaletteChanged();
}

} // namespace caelestia::services
