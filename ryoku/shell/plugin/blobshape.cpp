#include "blobshape.hpp"
#include "blobgroup.hpp"
#include "blobinvertedrect.hpp"
#include "blobrect.hpp"
#include "blobwave.hpp"

#include <qsggeometry.h>
#include <qsgnode.h>

#include <algorithm>
#include <cmath>

static float deformPadding(const QMatrix4x4& dm, float hw, float hh) {
    // bounding box of the deformed shape: |M * corners|
    const float dm00 = dm(0, 0), dm01 = dm(0, 1);
    const float dm10 = dm(1, 0), dm11 = dm(1, 1);
    const float boundX = std::abs(dm00) * hw + std::abs(dm01) * hh;
    const float boundY = std::abs(dm10) * hw + std::abs(dm11) * hh;
    const float extraX = std::max(boundX - hw, 0.0f) + std::abs(dm(0, 3));
    const float extraY = std::max(boundY - hh, 0.0f) + std::abs(dm(1, 3));
    return std::max(extraX, extraY);
}

static float cpuSdBox(float px, float py, float cx, float cy, float hw, float hh) {
    const float dx = std::abs(px - cx) - hw;
    const float dy = std::abs(py - cy) - hh;
    const float mdx = std::max(dx, 0.0f);
    const float mdy = std::max(dy, 0.0f);
    return std::sqrt(mdx * mdx + mdy * mdy) + std::min(std::max(dx, dy), 0.0f);
}

static float cpuSmoothstep(float edge0, float edge1, float x) {
    const float t = std::clamp((x - edge0) / (edge1 - edge0), 0.0f, 1.0f);
    return t * t * (3.0f - 2.0f * t);
}

static float cornerFillFactor(float sd, float smoothFactor) {
    // two-sided continuous window. corner squares (factor -> 0) only inside
    // ±smoothFactor of the neighbour's edge (the visible junction); full radius
    // far outside AND deep inside (where it's buried and squaring would crease
    // the interior). C0-continuous across sd = 0, so buried corners stop
    // collapsing to square.
    const float outside = cpuSmoothstep(0.0f, smoothFactor, sd);  // edge -> 0, far outside -> 1
    const float inside = cpuSmoothstep(0.0f, -smoothFactor, sd);  // edge -> 0, deep inside -> 1
    return std::max(outside, inside);
}

BlobShape::BlobShape(QQuickItem* parent)
    : QQuickItem(parent) {
    setFlag(ItemHasContents);
}

void BlobShape::setGroup(BlobGroup* g) {
    if (m_group == g)
        return;
    if (m_group && isComponentComplete())
        unregisterFromGroup();
    m_group = g;
    if (m_group && isComponentComplete())
        registerWithGroup();
    emit groupChanged();
    if (m_group)
        m_group->markDirty();
}

void BlobShape::setRadius(qreal r) {
    if (qFuzzyCompare(m_radius, r))
        return;
    m_radius = r;
    emit radiusChanged();
    if (m_group)
        m_group->markDirty();
}

void BlobShape::setSinks(bool s) {
    if (m_sinks == s)
        return;
    m_sinks = s;
    emit sinksChanged();
    if (m_group)
        m_group->markDirty();
}

void BlobShape::componentComplete() {
    QQuickItem::componentComplete();
    if (m_group)
        registerWithGroup();
}

void BlobShape::geometryChange(const QRectF& newGeometry, const QRectF& oldGeometry) {
    QQuickItem::geometryChange(newGeometry, oldGeometry);
    updateCenteredDeformMatrix();
    if (m_group) {
        // accumulate sub-pixel drift, else slow moves desync the shader
        m_pendingDx += static_cast<float>(newGeometry.x() - oldGeometry.x());
        m_pendingDy += static_cast<float>(newGeometry.y() - oldGeometry.y());
        const auto dw = std::abs(newGeometry.width() - oldGeometry.width());
        const auto dh = std::abs(newGeometry.height() - oldGeometry.height());
        if (std::abs(m_pendingDx) > 0.5f || std::abs(m_pendingDy) > 0.5f || dw > 0.5 || dh > 0.5) {
            m_pendingDx = 0;
            m_pendingDy = 0;
            m_group->markShapeDirty(this);
        }
    }
}

void BlobShape::updateCenteredDeformMatrix() {
    const auto cx = static_cast<float>(width()) * 0.5f;
    const auto cy = static_cast<float>(height()) * 0.5f;
    QMatrix4x4 result;
    result.translate(cx, cy);
    result *= m_deformMatrix;
    result.translate(-cx, -cy);
    if (m_centeredDeformMatrix != result) {
        m_centeredDeformMatrix = result;
        emit deformMatrixChanged();
    }
}

void BlobShape::cornerRadii(float out[4]) const {
    const auto maxR = static_cast<float>(std::min(width(), height())) * 0.5f;
    const auto r = std::min(static_cast<float>(m_radius), maxR);
    out[0] = r;
    out[1] = r;
    out[2] = r;
    out[3] = r;
}

void BlobShape::registerWithGroup() {
    if (m_group)
        m_group->addShape(this);
}

void BlobShape::unregisterFromGroup() {
    if (m_group)
        m_group->removeShape(this);
}

void BlobShape::updatePolish() {
    if (!m_group)
        return;

    // every shape gets up-to-date physics, once per frame
    m_group->ensurePhysicsUpdated();

    const QPointF scenePos = mapToScene(QPointF(0, 0));
    const float pad = static_cast<float>(m_group->smoothing());
    auto* selfWave = qobject_cast<BlobWave*>(this);

    if (isInvertedRect()) {
        m_cachedPaddedX = static_cast<float>(scenePos.x());
        m_cachedPaddedY = static_cast<float>(scenePos.y());
        m_cachedPaddedW = static_cast<float>(width());
        m_cachedPaddedH = static_cast<float>(height());
        m_localPaddedRect = QRectF(0, 0, width(), height());
    } else {
        const float hw = static_cast<float>(width()) * 0.5f;
        const float hh = static_cast<float>(height()) * 0.5f;
        const float wavePad = selfWave ? static_cast<float>(selfWave->amplitude()) : 0.0f;
        const float totalPad = pad + deformPadding(m_deformMatrix, hw, hh) + wavePad;

        m_cachedPaddedX = static_cast<float>(scenePos.x()) - totalPad;
        m_cachedPaddedY = static_cast<float>(scenePos.y()) - totalPad;
        m_cachedPaddedW = static_cast<float>(width()) + 2.0f * totalPad;
        m_cachedPaddedH = static_cast<float>(height()) + 2.0f * totalPad;
        m_localPaddedRect = QRectF(static_cast<double>(-totalPad), static_cast<double>(-totalPad),
            width() + 2.0 * static_cast<double>(totalPad), height() + 2.0 * static_cast<double>(totalPad));
    }

    // filter nearby normal rects
    m_cachedRects.clear();
    m_cachedMyIndex = -2;
    const QRectF myPadded(static_cast<double>(m_cachedPaddedX), static_cast<double>(m_cachedPaddedY),
        static_cast<double>(m_cachedPaddedW), static_cast<double>(m_cachedPaddedH));
    for (auto& info : m_cachedWaveInfo) {
        std::fill_n(info, 4, 0.0f);
        info[3] = -1.0f;
    }
    for (auto& data : m_cachedWaveData)
        std::fill_n(data, 64, 0.0f);
    std::fill_n(m_cachedWaves, 4, nullptr);

    // parallel array of shape ptrs so we can do pairwise exclusion lookups
    QVector<BlobShape*> rectShapes;
    rectShapes.reserve(m_group->shapes().size());
    int waveSlot = 0;

    for (BlobShape* other : m_group->shapes()) {
        if (other->isInvertedRect())
            continue;

        // skip zero-size
        if (other->width() <= 0 || other->height() <= 0)
            continue;

        if (isExcluded(other))
            continue;

        const QPointF otherScene = other->mapToScene(QPointF(0, 0));
        auto* otherWave = qobject_cast<BlobWave*>(other);

        bool include = false;
        if (isInvertedRect()) {
            include = true;
        } else {
            const float otherHW = static_cast<float>(other->width()) * 0.5f;
            const float otherHH = static_cast<float>(other->height()) * 0.5f;
            const float wavePad = otherWave ? static_cast<float>(otherWave->amplitude()) : 0.0f;
            const float otherPad = pad + deformPadding(other->m_deformMatrix, otherHW, otherHH) + wavePad;
            const QRectF otherPadded(otherScene.x() - static_cast<double>(otherPad),
                otherScene.y() - static_cast<double>(otherPad), other->width() + 2.0 * static_cast<double>(otherPad),
                other->height() + 2.0 * static_cast<double>(otherPad));
            include = myPadded.intersects(otherPadded);
        }

        if (include) {
            if (other == this)
                m_cachedMyIndex = static_cast<int>(m_cachedRects.size());

            const QMatrix4x4& dm = other->m_deformMatrix;
            const float a = dm(0, 0), b = dm(1, 0);
            const float c = dm(0, 1), d = dm(1, 1);

            BlobRectData r;
            r.cx = static_cast<float>(otherScene.x() + other->width() / 2.0);
            r.cy = static_cast<float>(otherScene.y() + other->height() / 2.0);
            r.hw = static_cast<float>(other->width() / 2.0);
            r.hh = static_cast<float>(other->height() / 2.0);
            other->cornerRadii(r.radius);
            r.offsetX = dm(0, 3);
            r.offsetY = dm(1, 3);

            // precompute inverse deformation matrix
            const float det = a * d - c * b;
            const float invDet = std::abs(det) > 1e-6f ? 1.0f / det : 1.0f;
            r.invDeform[0] = d * invDet;
            r.invDeform[1] = -b * invDet;
            r.invDeform[2] = -c * invDet;
            r.invDeform[3] = a * invDet;

            // precompute min eigenvalue, avoids per-pixel sqrt
            const float halfTr = 0.5f * (a + d);
            const float halfDiff = 0.5f * (a - d);
            r.minEig = halfTr - std::sqrt(halfDiff * halfDiff + c * c);

            // precompute screen-space AABB half-extents
            r.screenHalfX = std::abs(a) * r.hw + std::abs(c) * r.hh;
            r.screenHalfY = std::abs(b) * r.hw + std::abs(d) * r.hh;
            if (otherWave)
                r.screenHalfY += static_cast<float>(otherWave->amplitude());
            r.sinkWeight = other->m_sinks ? 1.0f : 0.0f;

            const auto requestedTaper = static_cast<float>(static_cast<const BlobRect*>(other)->taper());
            const float bottomRadius = std::max(std::max(r.radius[1], r.radius[2]), 0.0f);
            const float taperLimit = std::max(r.hw - bottomRadius, 0.0f);
            const float maxTaper = taperLimit > 0.0f ? std::nextafter(taperLimit, 0.0f) : 0.0f;
            r.taper = std::clamp(requestedTaper, -maxTaper, maxTaper);

            m_cachedRects.append(r);
            rectShapes.append(other);

            if (otherWave && m_group->waves().contains(otherWave) && m_cachedRects.size() <= 16 && waveSlot < 4) {
                const int count = otherWave->count();
                auto& info = m_cachedWaveInfo[waveSlot];
                info[0] = static_cast<float>(otherScene.x()) + otherWave->spanStart();
                info[1] = static_cast<float>(otherScene.x()) + otherWave->spanEnd();
                info[2] = static_cast<float>(otherWave->stepped() ? -count : count);
                info[3] = static_cast<float>(m_cachedRects.size() - 1);
                std::copy_n(otherWave->waveHeights().begin(), count, m_cachedWaveData[waveSlot]);
                m_cachedWaves[waveSlot] = otherWave;
                ++waveSlot;
            }
        }
    }

    if (isInvertedRect())
        m_cachedMyIndex = -1;

    // pairwise exclude masks. bit j in entry i = set iff rect i excludes j or
    // j excludes i. shader uses these to skip smin between excluded pairs.
    const auto cachedCount = m_cachedRects.size();
    for (qsizetype i = 0; i < cachedCount; ++i) {
        int mask = 0;
        BlobShape* si = rectShapes[i];
        for (qsizetype j = 0; j < cachedCount; ++j) {
            if (j == i)
                continue;
            BlobShape* sj = rectShapes[j];
            if (si->isExcluded(sj) || sj->isExcluded(si))
                mask |= (1 << j);
        }
        m_cachedRects[i].excludeMask = mask;
    }

    // cache inverted rect data
    m_cachedHasInverted = false;
    m_cachedInvertedRadius = 0;
    memset(m_cachedInvertedOuter, 0, sizeof(m_cachedInvertedOuter));
    memset(m_cachedInvertedInner, 0, sizeof(m_cachedInvertedInner));

    auto* inv = m_group->invertedRect();
    if (inv) {
        const QPointF invScene = inv->mapToScene(QPointF(0, 0));
        const float outerCX = static_cast<float>(invScene.x() + inv->width() / 2.0);
        const float outerCY = static_cast<float>(invScene.y() + inv->height() / 2.0);
        const float outerHW = static_cast<float>(inv->width() / 2.0);
        const float outerHH = static_cast<float>(inv->height() / 2.0);

        const float innerCX = outerCX + static_cast<float>((inv->borderLeft() - inv->borderRight()) / 2.0);
        const float innerCY = outerCY + static_cast<float>((inv->borderTop() - inv->borderBottom()) / 2.0);
        const float innerHW = outerHW - static_cast<float>((inv->borderLeft() + inv->borderRight()) / 2.0);
        const float innerHH = outerHH - static_cast<float>((inv->borderTop() + inv->borderBottom()) / 2.0);

        // near the border? within 2x smoothing of the inner edge
        bool nearBorder = isInvertedRect();
        if (!nearBorder) {
            const float margin = pad * 2.0f;
            const float myCX = m_cachedPaddedX + m_cachedPaddedW * 0.5f;
            const float myCY = m_cachedPaddedY + m_cachedPaddedH * 0.5f;
            const float myHW = m_cachedPaddedW * 0.5f;
            const float myHH = m_cachedPaddedH * 0.5f;
            // near-border = any edge of the padded rect within margin of the inner edge
            nearBorder = (myCX - myHW < innerCX - innerHW + margin) || (myCX + myHW > innerCX + innerHW - margin) ||
                         (myCY - myHH < innerCY - innerHH + margin) || (myCY + myHH > innerCY + innerHH - margin);
        }

        if (nearBorder) {
            m_cachedHasInverted = true;
            m_cachedInvertedRadius = static_cast<float>(inv->radius());

            m_cachedInvertedOuter[0] = outerCX;
            m_cachedInvertedOuter[1] = outerCY;
            m_cachedInvertedOuter[2] = outerHW;
            m_cachedInvertedOuter[3] = outerHH;

            m_cachedInvertedInner[0] = innerCX;
            m_cachedInvertedInner[1] = innerCY;
            m_cachedInvertedInner[2] = innerHW;
            m_cachedInvertedInner[3] = innerHH;
        }
    }

    // precompute effective per-corner radii: moves the O(N²) work off the GPU.
    const float smoothFactor = pad;
    constexpr float minR = 2.0f;
    const auto rectCount = m_cachedRects.size();
    for (qsizetype i = 0; i < rectCount; ++i) {
        auto& ri = m_cachedRects[i];
        const int riExcludeMask = ri.excludeMask;
        float fTr = 1.0f, fBr = 1.0f, fBl = 1.0f, fTl = 1.0f;

        const float cTrX = ri.cx + ri.hw, cTrY = ri.cy - ri.hh;
        const float cBrX = ri.cx + ri.hw, cBrY = ri.cy + ri.hh;
        const float cBlX = ri.cx - ri.hw, cBlY = ri.cy + ri.hh;
        const float cTlX = ri.cx - ri.hw, cTlY = ri.cy - ri.hh;

        for (qsizetype j = 0; j < rectCount; ++j) {
            if (j == i)
                continue;
            if (riExcludeMask & (1 << j))
                continue;
            const auto& rj = m_cachedRects[j];
            fTr = std::min(fTr, cornerFillFactor(cpuSdBox(cTrX, cTrY, rj.cx, rj.cy, rj.hw, rj.hh), smoothFactor));
            fBr = std::min(fBr, cornerFillFactor(cpuSdBox(cBrX, cBrY, rj.cx, rj.cy, rj.hw, rj.hh), smoothFactor));
            fBl = std::min(fBl, cornerFillFactor(cpuSdBox(cBlX, cBlY, rj.cx, rj.cy, rj.hw, rj.hh), smoothFactor));
            fTl = std::min(fTl, cornerFillFactor(cpuSdBox(cTlX, cTlY, rj.cx, rj.cy, rj.hw, rj.hh), smoothFactor));
        }

        if (m_cachedHasInverted) {
            const float icx = m_cachedInvertedInner[0];
            const float icy = m_cachedInvertedInner[1];
            const float ihw = m_cachedInvertedInner[2];
            const float ihh = m_cachedInvertedInner[3];
            fTr = std::min(fTr, cpuSmoothstep(0.0f, smoothFactor, -cpuSdBox(cTrX, cTrY, icx, icy, ihw, ihh)));
            fBr = std::min(fBr, cpuSmoothstep(0.0f, smoothFactor, -cpuSdBox(cBrX, cBrY, icx, icy, ihw, ihh)));
            fBl = std::min(fBl, cpuSmoothstep(0.0f, smoothFactor, -cpuSdBox(cBlX, cBlY, icx, icy, ihw, ihh)));
            fTl = std::min(fTl, cpuSmoothstep(0.0f, smoothFactor, -cpuSdBox(cTlX, cTlY, icx, icy, ihw, ihh)));
        }

        // mix base radii with fill factors into effective per-corner radii
        ri.radius[0] = std::max(ri.radius[0] * fTr, minR);
        ri.radius[1] = std::max(ri.radius[1] * fBr, minR);
        ri.radius[2] = std::max(ri.radius[2] * fBl, minR);
        ri.radius[3] = std::max(ri.radius[3] * fTl, minR);
    }
}

bool BlobShape::refreshWaveData(BlobWave* const* waves, int count) {
    // Geometry and union caches are static while a wave settles. Refresh only
    // the height arrays uploaded by the material.
    bool changed = false;
    for (int slot = 0; slot < 4; ++slot) {
        auto* cachedWave = m_cachedWaves[slot];
        if (!cachedWave)
            continue;
        bool matches = false;
        for (int i = 0; i < count; ++i) {
            if (waves[i] == cachedWave) {
                matches = true;
                break;
            }
        }
        if (!matches)
            continue;
        const int waveCount = cachedWave->count();
        std::copy_n(cachedWave->waveHeights().begin(), waveCount, m_cachedWaveData[slot]);
        std::fill(m_cachedWaveData[slot] + waveCount, m_cachedWaveData[slot] + 64, 0.0f);
        changed = true;
    }
    return changed;
}

QSGNode* BlobShape::updatePaintNode(QSGNode* oldNode, UpdatePaintNodeData*) {
    if (!m_group) {
        delete oldNode;
        return nullptr;
    }

    auto* node = static_cast<QSGGeometryNode*>(oldNode);
    const bool created = !node;
    if (!node) {
        node = new QSGGeometryNode;

        auto* geometry = new QSGGeometry(QSGGeometry::defaultAttributes_TexturedPoint2D(), 4);
        geometry->setDrawingMode(QSGGeometry::DrawTriangleStrip);
        node->setGeometry(geometry);
        node->setFlag(QSGNode::OwnsGeometry);

        auto* material = new BlobMaterial;
        material->setFlag(QSGMaterial::Blending);
        node->setMaterial(material);
        node->setFlag(QSGNode::OwnsMaterial);
    }

    auto* geometry = node->geometry();
    auto* v = geometry->vertexDataAsTexturedPoint2D();

    const float x0 = static_cast<float>(m_localPaddedRect.x());
    const float y0 = static_cast<float>(m_localPaddedRect.y());
    const float x1 = x0 + static_cast<float>(m_localPaddedRect.width());
    const float y1 = y0 + static_cast<float>(m_localPaddedRect.height());

    if (created || v[0].x != x0 || v[0].y != y0 || v[1].x != x1 || v[2].y != y1) {
        v[0].set(x0, y0, 0.0f, 0.0f);
        v[1].set(x1, y0, 1.0f, 0.0f);
        v[2].set(x0, y1, 0.0f, 1.0f);
        v[3].set(x1, y1, 1.0f, 1.0f);
        node->markDirty(QSGNode::DirtyGeometry);
    }

    auto* material = static_cast<BlobMaterial*>(node->material());
    material->m_paddedX = m_cachedPaddedX;
    material->m_paddedY = m_cachedPaddedY;
    material->m_paddedW = m_cachedPaddedW;
    material->m_paddedH = m_cachedPaddedH;
    material->m_smoothFactor = static_cast<float>(m_group->smoothing());
    material->m_myIndex = m_cachedMyIndex;
    material->m_color = m_group->color();
    material->m_hasInverted = m_cachedHasInverted ? 1 : 0;
    material->m_invertedRadius = m_cachedInvertedRadius;
    material->m_shadowStrength = static_cast<float>(m_group->shadowStrength());
    material->m_shadowSize = static_cast<float>(m_group->shadowSize());
    material->m_borderColor = m_group->borderColor();
    material->m_borderWidth = static_cast<float>(m_group->borderWidth());
    material->m_cutCorners = m_group->cutCorners() ? 1.0f : 0.0f;
    material->m_waveColor = m_group->waveColor();
    memcpy(material->m_invertedOuter, m_cachedInvertedOuter, sizeof(m_cachedInvertedOuter));
    memcpy(material->m_invertedInner, m_cachedInvertedInner, sizeof(m_cachedInvertedInner));
    memcpy(material->m_waveInfo, m_cachedWaveInfo, sizeof(m_cachedWaveInfo));
    memcpy(material->m_waveData, m_cachedWaveData, sizeof(m_cachedWaveData));

    const int count = static_cast<int>(qMin(m_cachedRects.size(), qsizetype(16)));
    material->m_rectCount = count;
    for (int i = 0; i < count; ++i)
        material->m_rects[i] = m_cachedRects[i];

    node->markDirty(QSGNode::DirtyMaterial);

    return node;
}
