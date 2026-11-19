#include "blobgroup.hpp"
#include "blobinvertedrect.hpp"
#include "blobwave.hpp"
#include "blobshape.hpp"

#include <algorithm>
#include <utility>

BlobGroup::BlobGroup(QObject* parent)
    : QObject(parent) {
    m_waveTimer.setSingleShot(true);
    m_waveTimer.setTimerType(Qt::PreciseTimer);
    connect(&m_waveTimer, &QTimer::timeout, this, &BlobGroup::flushWaveUpdates);
}

BlobGroup::~BlobGroup() {
    for (auto* shape : std::as_const(m_shapes))
        shape->m_group = nullptr;
    if (m_invertedRect)
        static_cast<BlobShape*>(m_invertedRect)->m_group = nullptr;
}

void BlobGroup::setSmoothing(qreal s) {
    if (qFuzzyCompare(m_smoothing, s))
        return;
    m_smoothing = s;
    emit smoothingChanged();
    markDirty();
}

void BlobGroup::setColor(const QColor& c) {
    if (m_color == c)
        return;
    m_color = c;
    emit colorChanged();
    markDirty();
}

void BlobGroup::setShadowStrength(qreal v) {
    if (m_shadowStrength == v)
        return;
    m_shadowStrength = v;
    emit shadowStrengthChanged();
    markDirty();
}

void BlobGroup::setShadowSize(qreal v) {
    if (m_shadowSize == v)
        return;
    m_shadowSize = v;
    emit shadowSizeChanged();
    markDirty();
}

void BlobGroup::setBorderColor(const QColor& c) {
    if (m_borderColor == c)
        return;
    m_borderColor = c;
    emit borderColorChanged();
    markDirty();
}

void BlobGroup::setBorderWidth(qreal v) {
    if (m_borderWidth == v)
        return;
    m_borderWidth = v;
    emit borderWidthChanged();
    markDirty();
}

void BlobGroup::setWaveColor(const QColor& c) {
    if (m_waveColor == c)
        return;
    m_waveColor = c;
    emit waveColorChanged();
    markDirty();
}

void BlobGroup::setCutCorners(bool v) {
    if (m_cutCorners == v)
        return;
    m_cutCorners = v;
    emit cutCornersChanged();
    markDirty();
}

void BlobGroup::addShape(BlobShape* shape) {
    if (!shape || m_shapes.contains(shape))
        return;
    m_shapes.append(shape);
    markDirty();
}

void BlobGroup::removeShape(BlobShape* shape) {
    m_shapes.removeOne(shape);
    markDirty();
}

void BlobGroup::setInvertedRect(BlobInvertedRect* rect) {
    if (m_invertedRect == rect)
        return;
    m_invertedRect = rect;
    markDirty();
}

void BlobGroup::clearInvertedRect(BlobInvertedRect* rect) {
    if (m_invertedRect != rect)
        return;
    m_invertedRect = nullptr;
    markDirty();
}

void BlobGroup::addWave(BlobWave* wave) {
    if (!wave || m_waves.contains(wave))
        return;
    if (m_waves.size() >= 4) {
        qWarning() << "BlobGroup accepts at most four BlobWaves";
        return;
    }
    m_waves.append(wave);
    markDirty();
}

void BlobGroup::removeWave(BlobWave* wave) {
    if (m_waves.removeOne(wave))
        markDirty();
}

void BlobGroup::markDirty() {
    m_physicsUpdated = false;
    for (auto* shape : std::as_const(m_shapes)) {
        shape->polish();
        shape->update();
    }
    if (m_invertedRect) {
        static_cast<BlobShape*>(m_invertedRect)->polish();
        static_cast<BlobShape*>(m_invertedRect)->update();
    }
}

void BlobGroup::markShapeDirty(BlobShape* source) {
    m_physicsUpdated = false;

    source->polish();
    source->update();

    // cached padded rects = neighbor lookup
    const float pad = static_cast<float>(m_smoothing) * 2.0f;
    const QRectF srcRect(static_cast<double>(source->m_cachedPaddedX - pad),
        static_cast<double>(source->m_cachedPaddedY - pad), static_cast<double>(source->m_cachedPaddedW + pad * 2.0f),
        static_cast<double>(source->m_cachedPaddedH + pad * 2.0f));

    for (auto* shape : std::as_const(m_shapes)) {
        if (shape == source)
            continue;
        const QRectF otherRect(static_cast<double>(shape->m_cachedPaddedX), static_cast<double>(shape->m_cachedPaddedY),
            static_cast<double>(shape->m_cachedPaddedW), static_cast<double>(shape->m_cachedPaddedH));
        if (srcRect.intersects(otherRect)) {
            shape->polish();
            shape->update();
        }
    }

    if (m_invertedRect && static_cast<BlobShape*>(m_invertedRect) != source) {
        static_cast<BlobShape*>(m_invertedRect)->polish();
        static_cast<BlobShape*>(m_invertedRect)->update();
    }
}

void BlobGroup::requestWaveUpdate() {
    if (m_waveTimer.isActive())
        return;
    // The analyser updates at 30 Hz, so a 165 Hz screen must not turn spring
    // settling into a full-rate repaint loop.
    constexpr int MinimumWaveFrameMs = 17;
    const int delay = m_waveClock.isValid()
        ? std::max(0, MinimumWaveFrameMs - static_cast<int>(m_waveClock.elapsed()))
        : 0;
    m_waveTimer.start(delay);
}

void BlobGroup::flushWaveUpdates() {
    m_physicsUpdated = false;
    m_updateWavePhysics = true;
    ensurePhysicsUpdated();
    m_updateWavePhysics = false;

    BlobWave* changed[4] = {};
    int changedCount = 0;
    bool needsNextFrame = false;
    for (auto* wave : std::as_const(m_waves)) {
        if (wave->takeVisualChange() && changedCount < 4)
            changed[changedCount++] = wave;
        needsNextFrame = needsNextFrame || wave->activePhysics();
    }
    if (changedCount > 0)
        markWaveDataDirty(changed, changedCount);

    if (m_waveClock.isValid())
        m_waveClock.restart();
    else
        m_waveClock.start();
    if (needsNextFrame)
        requestWaveUpdate();
}

void BlobGroup::markWaveDataDirty(BlobWave* const* waves, int count) {
    for (auto* shape : std::as_const(m_shapes)) {
        if (shape->refreshWaveData(waves, count))
            shape->update();
    }
    if (m_invertedRect) {
        auto* inverted = static_cast<BlobShape*>(m_invertedRect);
        if (inverted->refreshWaveData(waves, count))
            inverted->update();
    }
}

void BlobGroup::ensurePhysicsUpdated() {
    if (m_physicsUpdated)
        return;
    m_physicsUpdated = true;
    for (auto* shape : std::as_const(m_shapes)) {
        if (!m_updateWavePhysics && qobject_cast<BlobWave*>(shape))
            continue;
        shape->updatePhysics();
    }
}
