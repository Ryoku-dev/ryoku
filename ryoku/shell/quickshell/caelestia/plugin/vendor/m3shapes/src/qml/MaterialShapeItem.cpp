#include "MaterialShapeItem.hpp"
#include "../core/RoundedPolygon.hpp"
#include "../shapes/Shapes.hpp"
#include "SmoothShapeMaterial.hpp"
#include <QSGGeometry>
#include <QSGGeometryNode>
#include <QSGNode>
#include <QVariantMap>
#include <algorithm>
#include <cmath>
#include <cstring>
#include <numbers>
#include <vector>

using namespace RoundedPolygon;

// ========== RoundedPolygonWrapper ==========

RoundedPolygonWrapper::RoundedPolygonWrapper(const RoundedPolygonShape& shape)
    : m_shape(shape) {}

const RoundedPolygonShape& RoundedPolygonWrapper::shape() const {
    static RoundedPolygonShape defaultShape =
        MaterialShapes::getShape(MaterialShapes::ShapeType::Circle);
    return m_shape.has_value() ? *m_shape : defaultShape;
}

RoundedPolygonWrapper RoundedPolygonWrapper::normalized() const {
    if (!m_shape.has_value()) {
        return {};
    }
    return RoundedPolygonWrapper(m_shape->normalized());
}

// ========== MaterialShapeItem ==========

MaterialShapeItem::MaterialShapeItem(QQuickItem* parent)
    : QQuickItem(parent) {
    setFlag(ItemHasContents, true);

    // M3 expressive fast spatial
    m_animationEasing.setType(QEasingCurve::BezierSpline);
    m_animationEasing.addCubicBezierSegment(
        QPointF(0.42, 1.67), QPointF(0.21, 0.90), QPointF(1.0, 1.0));

    m_animation = new QPropertyAnimation(this);
    m_animation->setTargetObject(this);
    m_animation->setPropertyName("morphProgress");
    m_animation->setStartValue(0.0f);
    m_animation->setEndValue(1.0f);
    m_animation->setDuration(m_animationDuration);
    m_animation->setEasingCurve(m_animationEasing);

    connect(m_animation, &QPropertyAnimation::valueChanged, this,
        &MaterialShapeItem::onAnimationValueChanged);
    connect(m_animation, &QPropertyAnimation::finished, this,
        &MaterialShapeItem::onMorphFinished);

    // Initialize with circle shape
    auto circleShape =
        MaterialShapes::getShape(MaterialShapes::ShapeType::Circle);
    m_morph = std::make_unique<Morph>(circleShape, circleShape);
}

// ========== Factory functions ==========

QVariantMap MaterialShapeItem::point(
    float x, float y, float radius, float smoothing) {
    QVariantMap map;
    map["x"] = x;
    map["y"] = y;
    map["radius"] = radius;
    map["smoothing"] = smoothing;
    return map;
}

RoundedPolygonWrapper MaterialShapeItem::polygon(const QVariantList& vertices,
    int reps, float centerX, float centerY, bool mirroring) {
    if (vertices.isEmpty()) {
        return {};
    }

    std::vector<MaterialShapes::PointNRound> points;
    points.reserve(static_cast<size_t>(vertices.size()));

    for (const QVariant& vertex : vertices) {
        float x = 0.0f, y = 0.0f;
        float radius = 0.0f;
        float smoothing = 0.0f;

        if (vertex.canConvert<QVariantMap>()) {
            QVariantMap map = vertex.toMap();
            x = map.value("x", 0.0f).toFloat();
            y = map.value("y", 0.0f).toFloat();
            radius = map.value("radius", 0.0f).toFloat();
            smoothing = map.value("smoothing", 0.0f).toFloat();
        } else if (vertex.canConvert<QPointF>()) {
            QPointF pt = vertex.toPointF();
            x = static_cast<float>(pt.x());
            y = static_cast<float>(pt.y());
        } else if (vertex.canConvert<QVariantList>()) {
            QVariantList coords = vertex.toList();
            if (coords.size() >= 2) {
                x = coords[0].toFloat();
                y = coords[1].toFloat();
            }
        }

        points.emplace_back(x, y, CornerRounding(radius, smoothing));
    }

    return RoundedPolygonWrapper(
        MaterialShapes::customPolygon(points, reps, centerX, centerY, mirroring)
            .normalized());
}

RoundedPolygonWrapper MaterialShapeItem::regularPolygon(
    int numVertices, float radius, float smoothing) {
    if (numVertices < 3) {
        numVertices = 3;
    }
    return RoundedPolygonWrapper(RoundedPolygonShape(
        numVertices, 1.0f, 0.0f, 0.0f, CornerRounding(radius, smoothing))
            .normalized());
}

RoundedPolygonWrapper MaterialShapeItem::star(
    int points, float innerRadius, float radius, float smoothing) {
    if (points < 2) {
        points = 2;
    }
    return RoundedPolygonWrapper(Shapes::star(
        points, 1.0f, innerRadius, CornerRounding(radius, smoothing))
            .normalized());
}

RoundedPolygonWrapper MaterialShapeItem::rectangle(
    float width, float height, float radius, float smoothing) {
    return RoundedPolygonWrapper(
        Shapes::rectangle(width, height, CornerRounding(radius, smoothing))
            .normalized());
}

RoundedPolygonWrapper MaterialShapeItem::squircle(float n, int segments) {
    if (n < 0.1f) {
        n = 0.1f;
    }
    if (segments < 4) {
        segments = 4;
    }

    std::vector<float> vertices;
    vertices.reserve(static_cast<size_t>(segments) * 2);

    const float exp = 2.0f / n;
    const float pi2 = static_cast<float>(std::numbers::pi) * 2.0f;

    for (int i = 0; i < segments; ++i) {
        float t = pi2 * static_cast<float>(i) / static_cast<float>(segments);
        float cosT = std::cos(t);
        float sinT = std::sin(t);

        // Superellipse: |x|^n + |y|^n = 1
        // Parametric: x = sign(cos(t)) * |cos(t)|^(2/n)
        //             y = sign(sin(t)) * |sin(t)|^(2/n)
        float x = std::copysign(std::pow(std::abs(cosT), exp), cosT);
        float y = std::copysign(std::pow(std::abs(sinT), exp), sinT);

        // Scale to 0-1 range (centered at 0.5)
        vertices.push_back(x * 0.5f + 0.5f);
        vertices.push_back(y * 0.5f + 0.5f);
    }

    // No corner rounding needed - the curve itself is smooth
    return RoundedPolygonWrapper(
        RoundedPolygonShape(vertices, CornerRounding::Unrounded).normalized());
}

// ========== Property setters ==========

void MaterialShapeItem::setShape(Shape shape) {
    if (m_targetShape != shape) {
        m_targetShape = shape;
        emit shapeChanged();

        if (m_animation->state() == QAbstractAnimation::Running) {
            m_animation->stop();
        }

        // Skip morphing during initial creation
        if (!isComponentComplete()) {
            m_currentShape = shape;
            m_fromShape = shape;
            m_toShape = shape;
            auto targetShape = getShapeForEnum(shape);
            m_morph = std::make_unique<Morph>(targetShape, targetShape);
            m_morphProgress = 1.0f;
            return;
        }

        // Start from toShape (handles transition from manual to auto mode)
        startMorph(m_toShape, shape);
    }
}

void MaterialShapeItem::setAnimationDuration(int duration) {
    if (m_animationDuration != duration) {
        m_animationDuration = duration;
        m_animation->setDuration(duration);
        emit animationDurationChanged();
    }
}

void MaterialShapeItem::setAnimationEasing(const QEasingCurve& easing) {
    if (m_animationEasing != easing) {
        m_animationEasing = easing;
        m_animation->setEasingCurve(easing);
        emit animationEasingChanged();
    }
}

void MaterialShapeItem::setFromShape(Shape shape) {
    if (m_fromShape != shape) {
        m_fromShape = shape;
        emit fromShapeChanged();
        rebuildMorph();
    }
}

void MaterialShapeItem::setToShape(Shape shape) {
    if (m_toShape != shape) {
        m_toShape = shape;
        emit toShapeChanged();
        rebuildMorph();
    }
}

void MaterialShapeItem::setCustomShape(const RoundedPolygonWrapper& shape) {
    if (!shape.isValid()) {
        return;
    }

    m_customShape = shape;
    emit customShapeChanged();

    // Only rebuild morph if shape is already Custom
    if (m_targetShape == Custom) {
        if (!isComponentComplete()) {
            m_morph = std::make_unique<Morph>(shape.shape(), shape.shape());
            m_morphProgress = 1.0f;
        } else {
            rebuildMorph();
        }
        invalidatePath();
    }
}

void MaterialShapeItem::setCustomFromShape(const RoundedPolygonWrapper& shape) {
    if (!shape.isValid()) {
        return;
    }
    m_customFromShape = shape;
    m_fromShape = Custom;
    emit customFromShapeChanged();
    emit fromShapeChanged();
    rebuildMorph();
}

void MaterialShapeItem::setCustomToShape(const RoundedPolygonWrapper& shape) {
    if (!shape.isValid()) {
        return;
    }
    m_customToShape = shape;
    m_toShape = Custom;
    emit customToShapeChanged();
    emit toShapeChanged();
    rebuildMorph();
}

void MaterialShapeItem::rebuildMorph() {
    if (m_batchDepth > 0) {
        m_pendingRebuild = true;
        return;
    }
    RoundedPolygonShape from = getShapeForEnum(m_fromShape);
    RoundedPolygonShape to = getShapeForEnum(m_toShape);
    m_morph = std::make_unique<Morph>(from, to);
    invalidatePath();
}

void MaterialShapeItem::beginBatchUpdate() {
    ++m_batchDepth;
}

void MaterialShapeItem::endBatchUpdate() {
    if (m_batchDepth <= 0) {
        return;
    }
    if (--m_batchDepth == 0 && m_pendingRebuild) {
        m_pendingRebuild = false;
        rebuildMorph();
    }
}

RoundedPolygonShape MaterialShapeItem::getShapeForEnum(Shape shape) const {
    if (shape == Custom) {
        // Check which custom shape to use based on context
        if (m_customToShape.isValid() && shape == m_toShape) {
            return m_customToShape.shape();
        }
        if (m_customFromShape.isValid() && shape == m_fromShape) {
            return m_customFromShape.shape();
        }
        if (m_customShape.isValid()) {
            return m_customShape.shape();
        }
        // Fallback to circle
        return MaterialShapes::getShape(MaterialShapes::ShapeType::Circle);
    }
    return MaterialShapes::getShape(
        static_cast<MaterialShapes::ShapeType>(shape));
}

void MaterialShapeItem::startMorph(Shape from, Shape to) {
    // Sync manual morph properties
    m_fromShape = from;
    m_toShape = to;
    emit fromShapeChanged();
    emit toShapeChanged();

    auto fromShape = getShapeForEnum(from);
    auto toShape = getShapeForEnum(to);

    m_morph = std::make_unique<Morph>(fromShape, toShape);
    m_morphProgress = 0.0f;

    m_animation->start();

    invalidatePath();
}

void MaterialShapeItem::onAnimationValueChanged(const QVariant& value) {
    setMorphProgress(value.toFloat());
}

void MaterialShapeItem::setMorphProgress(float progress) {
    if (!qFuzzyCompare(m_morphProgress, progress)) {
        m_morphProgress = progress;
        emit morphProgressChanged();
        invalidatePath();
    }
}

void MaterialShapeItem::onMorphFinished() {
    m_currentShape = m_targetShape;
    m_fromShape = m_targetShape;
    m_morphProgress = 1.0f;
    m_geometryDirty = true;
    emit fromShapeChanged();
    update();
}

void MaterialShapeItem::setColor(const QColor& color) {
    if (m_color != color) {
        m_color = color;
        m_geometryDirty = true;
        emit colorChanged();
        update();
    }
}

void MaterialShapeItem::setImplicitSize(qreal size) {
    if (!qFuzzyCompare(m_implicitSize, size)) {
        m_implicitSize = size;
        setImplicitWidth(size);
        setImplicitHeight(size);
        emit implicitSizeChanged();
    }
}

void MaterialShapeItem::setStrokeColor(const QColor& color) {
    if (m_strokeColor != color) {
        m_strokeColor = color;
        m_geometryDirty = true;
        emit strokeColorChanged();
        update();
    }
}

void MaterialShapeItem::setStrokeWidth(float width) {
    if (!qFuzzyCompare(m_strokeWidth, width)) {
        m_strokeWidth = width;
        m_geometryDirty = true;
        emit strokeWidthChanged();
        update();
    }
}

QPainterPath MaterialShapeItem::buildPath() const {
    QPainterPath path;

    if (m_morph == nullptr) {
        return path;
    }

    auto cubics = m_morph->asCubics(m_morphProgress);

    if (cubics.empty()) {
        return path;
    }

    float itemWidth = static_cast<float>(width());
    float itemHeight = static_cast<float>(height());
    float size = std::min(itemWidth, itemHeight);
    float cX = itemWidth / 2.0f;
    float cY = itemHeight / 2.0f;

    auto transformPoint = [&](float px, float py) -> QPointF {
        float x = cX + (px - 0.5f) * size;
        float y = cY + (py - 0.5f) * size;
        return QPointF(static_cast<qreal>(x), static_cast<qreal>(y));
    };

    // Build path from cubics (like Android's toPath())
    bool first = true;
    for (const auto& cubic : cubics) {
        if (first) {
            path.moveTo(transformPoint(cubic.anchor0X(), cubic.anchor0Y()));
            first = false;
        }
        path.cubicTo(transformPoint(cubic.control0X(), cubic.control0Y()),
            transformPoint(cubic.control1X(), cubic.control1Y()),
            transformPoint(cubic.anchor1X(), cubic.anchor1Y()));
    }
    path.closeSubpath();

    return path;
}

const QPainterPath& MaterialShapeItem::cachedPath() const {
    if (m_pathDirty) {
        m_cachedPath = buildPath();
        m_pathDirty = false;
        m_polygonsDirty = true;
    }
    return m_cachedPath;
}

const QList<QPolygonF>& MaterialShapeItem::cachedPolygons() const {
    if (m_polygonsDirty) {
        m_cachedPolygons = cachedPath().toSubpathPolygons();
        m_polygonsDirty = false;
    }
    return m_cachedPolygons;
}

void MaterialShapeItem::invalidatePath() {
    m_pathDirty = true;
    m_polygonsDirty = true;
    m_geometryDirty = true;
    update();
}

void MaterialShapeItem::geometryChange(
    const QRectF& newGeometry, const QRectF& oldGeometry) {
    if (newGeometry.size() != oldGeometry.size()) {
        m_pathDirty = true;
        m_polygonsDirty = true;
        m_geometryDirty = true;
        update();
    }
    QQuickItem::geometryChange(newGeometry, oldGeometry);
}

qreal MaterialShapeItem::rayHitDistance(qreal dx, qreal dy) const {
    if (width() <= 0 || height() <= 0) {
        return -1.0;
    }
    const QList<QPolygonF>& polygons = cachedPolygons();
    if (polygons.isEmpty()) {
        return -1.0;
    }

    const QPointF center(width() / 2.0, height() / 2.0);

    // Analytically intersect each flattened edge with the ray. Take the
    // farthest hit so non-convex shapes return their outer boundary.
    qreal bestT = -1.0;
    for (const QPolygonF& poly : polygons) {
        const int n = static_cast<int>(poly.size());
        if (n < 2) {
            continue;
        }
        for (int i = 0; i < n - 1; ++i) {
            const QPointF& a = poly[i];
            const QPointF& b = poly[i + 1];
            const qreal ex = b.x() - a.x();
            const qreal ey = b.y() - a.y();
            const qreal denom = ex * dy - ey * dx;
            if (std::abs(denom) < 1e-9) {
                continue;
            }
            const qreal rx = a.x() - center.x();
            const qreal ry = a.y() - center.y();
            const qreal t = (ex * ry - ey * rx) / denom;
            const qreal s = (dx * ry - dy * rx) / denom;
            if (t < 0.0 || s < 0.0 || s > 1.0) {
                continue;
            }
            if (t > bestT) {
                bestT = t;
            }
        }
    }
    return bestT;
}

QPointF MaterialShapeItem::pointAtAngle(qreal angleDegrees) const {
    // Input angle is in the parent (screen) frame. Subtract the item's
    // rotation so the ray is cast in the path's local frame; the returned
    // local point then re-rotates correctly when used as a child of this
    // item.
    const qreal radians =
        (angleDegrees - rotation()) * std::numbers::pi / 180.0;
    const qreal dx = std::sin(radians);
    const qreal dy = -std::cos(radians);

    const qreal t = rayHitDistance(dx, dy);
    if (t < 0.0) {
        return {};
    }
    const QPointF center(width() / 2.0, height() / 2.0);
    return QPointF(center.x() + dx * t, center.y() + dy * t);
}

qreal MaterialShapeItem::distanceAtAngle(qreal angleDegrees) const {
    const qreal radians =
        (angleDegrees - rotation()) * std::numbers::pi / 180.0;
    const qreal dx = std::sin(radians);
    const qreal dy = -std::cos(radians);
    const qreal t = rayHitDistance(dx, dy);
    return t < 0.0 ? 0.0 : t;
}

QRectF MaterialShapeItem::pathBounds() const {
    if (width() <= 0 || height() <= 0) {
        return {};
    }
    return cachedPath().boundingRect();
}

bool MaterialShapeItem::contains(const QPointF& point) const {
    if (width() <= 0 || height() <= 0) {
        return false;
    }
    return cachedPath().contains(point);
}

namespace {

// ========== Scene-graph geometry helpers ==========
//
// Rendering is resolution-independent: the morphed cubic contour is flattened
// into a triangle mesh in item-local coordinates and drawn through the scene
// graph. Unlike the old QQuickPaintedItem (which rasterised into a fixed-size
// texture that QML then stretched), the mesh is re-rasterised at the final
// on-screen resolution, so the solid body stays crisp at any scale.
//
// Edge antialiasing is handled by SmoothShapeMaterial's shader: every contour
// vertex is duplicated into an inner (full-colour) and an outer (transparent)
// vertex carrying an outward feather DIRECTION. The vertex shader expands that
// direction by a fixed DEVICE-pixel amount AFTER the combined matrix, so the
// antialiased fringe is always ~1 screen pixel wide regardless of the item's
// scale, any accumulated parent scale, or the device pixel ratio -- none of
// which the CPU side needs to know or react to.

double distSq(const QPointF& a, const QPointF& b) {
    const double dx = a.x() - b.x();
    const double dy = a.y() - b.y();
    return dx * dx + dy * dy;
}

bool cubicIsFlat(const QPointF& p0, const QPointF& p1, const QPointF& p2,
    const QPointF& p3, double tol) {
    const double ux = p3.x() - p0.x();
    const double uy = p3.y() - p0.y();
    const double l2 = ux * ux + uy * uy;
    if (l2 < 1e-12) {
        // Near-zero chord: flat unless a control point strays far.
        return distSq(p0, p1) <= tol * tol && distSq(p0, p2) <= tol * tol;
    }
    const double d1 = std::abs((p1.x() - p0.x()) * uy - (p1.y() - p0.y()) * ux);
    const double d2 = std::abs((p2.x() - p0.x()) * uy - (p2.y() - p0.y()) * ux);
    return (d1 + d2) * (d1 + d2) <= tol * tol * l2;
}

// Recursively flattens a cubic, appending the END point of each flat segment.
void flattenCubic(const QPointF& p0, const QPointF& p1, const QPointF& p2,
    const QPointF& p3, double tol, QList<QPointF>& out, int depth) {
    if (depth >= 18 || cubicIsFlat(p0, p1, p2, p3, tol)) {
        out.append(p3);
        return;
    }
    const QPointF p01 = (p0 + p1) / 2.0;
    const QPointF p12 = (p1 + p2) / 2.0;
    const QPointF p23 = (p2 + p3) / 2.0;
    const QPointF p012 = (p01 + p12) / 2.0;
    const QPointF p123 = (p12 + p23) / 2.0;
    const QPointF p0123 = (p012 + p123) / 2.0;
    flattenCubic(p0, p01, p012, p0123, tol, out, depth + 1);
    flattenCubic(p0123, p123, p23, p3, tol, out, depth + 1);
}

// Flattens the morphed cubic contour into a closed ring of item-local points.
QList<QPointF> buildRing(
    const std::vector<Cubic>& cubics, double itemW, double itemH) {
    QList<QPointF> ring;
    if (cubics.empty()) {
        return ring;
    }
    const double size = std::min(itemW, itemH);
    const double cX = itemW / 2.0;
    const double cY = itemH / 2.0;
    const auto tf = [&](float px, float py) {
        return QPointF(cX + (static_cast<double>(px) - 0.5) * size,
            cY + (static_cast<double>(py) - 0.5) * size);
    };
    // Flatness tolerance in item-local pixels. Kept tight so curves stay
    // smooth even when the item is scaled up several times.
    const double tol = 0.1;

    ring.append(tf(cubics.front().anchor0X(), cubics.front().anchor0Y()));
    for (const Cubic& c : cubics) {
        flattenCubic(tf(c.anchor0X(), c.anchor0Y()),
            tf(c.control0X(), c.control0Y()), tf(c.control1X(), c.control1Y()),
            tf(c.anchor1X(), c.anchor1Y()), tol, ring, 0);
    }

    // Drop the closing point (equals the first) and any near-duplicates so
    // triangulation stays well-conditioned.
    QList<QPointF> dedup;
    dedup.reserve(ring.size());
    for (const QPointF& p : std::as_const(ring)) {
        if (dedup.isEmpty() || distSq(dedup.last(), p) > 1e-6) {
            dedup.append(p);
        }
    }
    while (dedup.size() > 1 && distSq(dedup.first(), dedup.last()) <= 1e-6) {
        dedup.removeLast();
    }
    return dedup;
}

double signedCrossZ(const QPointF& a, const QPointF& b, const QPointF& c) {
    return (b.x() - a.x()) * (c.y() - a.y())
        - (b.y() - a.y()) * (c.x() - a.x());
}

bool pointInTriangle(
    const QPointF& p, const QPointF& a, const QPointF& b, const QPointF& c) {
    const double d1 = signedCrossZ(a, b, p);
    const double d2 = signedCrossZ(b, c, p);
    const double d3 = signedCrossZ(c, a, p);
    const bool hasNeg = d1 < 0 || d2 < 0 || d3 < 0;
    const bool hasPos = d1 > 0 || d2 > 0 || d3 > 0;
    return !(hasNeg && hasPos);
}

// Ear-clipping triangulation of a simple polygon, emitting index triples into
// ring. Handles concave shapes (e.g. Puffy, PixelTriangle) where a centroid
// fan would self-overlap; falls back to a fan if no ear is found.
void triangulate(const QList<QPointF>& ring, std::vector<int>& triangles) {
    const int n = static_cast<int>(ring.size());
    if (n < 3) {
        return;
    }
    double area = 0.0;
    for (int i = 0; i < n; ++i) {
        const QPointF& a = ring[i];
        const QPointF& b = ring[(i + 1) % n];
        area += a.x() * b.y() - b.x() * a.y();
    }
    // Normalise to a positive (CCW) ordering so convex corners test > 0.
    std::vector<int> v(static_cast<size_t>(n));
    for (int i = 0; i < n; ++i) {
        v[static_cast<size_t>(i)] = (area >= 0.0) ? i : (n - 1 - i);
    }

    int remaining = n;
    int guard = 0;
    while (remaining > 3 && guard++ < 4 * n) {
        bool earFound = false;
        for (int i = 0; i < remaining; ++i) {
            const int ip =
                v[static_cast<size_t>((i - 1 + remaining) % remaining)];
            const int ic = v[static_cast<size_t>(i)];
            const int inx = v[static_cast<size_t>((i + 1) % remaining)];
            const QPointF& a = ring[ip];
            const QPointF& b = ring[ic];
            const QPointF& c = ring[inx];
            if (signedCrossZ(a, b, c) <= 0.0) {
                continue; // reflex or collinear corner
            }
            bool contains = false;
            for (int j = 0; j < remaining; ++j) {
                const int vj = v[static_cast<size_t>(j)];
                if (vj == ip || vj == ic || vj == inx) {
                    continue;
                }
                if (pointInTriangle(ring[vj], a, b, c)) {
                    contains = true;
                    break;
                }
            }
            if (contains) {
                continue;
            }
            triangles.push_back(ip);
            triangles.push_back(ic);
            triangles.push_back(inx);
            v.erase(v.begin() + i);
            --remaining;
            earFound = true;
            break;
        }
        if (!earFound) {
            break;
        }
    }
    if (remaining == 3) {
        triangles.push_back(v[0]);
        triangles.push_back(v[1]);
        triangles.push_back(v[2]);
    } else if (remaining > 3) {
        for (int i = 1; i + 1 < remaining; ++i) {
            triangles.push_back(v[0]);
            triangles.push_back(v[static_cast<size_t>(i)]);
            triangles.push_back(v[static_cast<size_t>(i + 1)]);
        }
    }
}

// Outward unit normal at each ring vertex (edge-normal bisector, flipped to
// point away from the centroid). Used for the AA fringe and stroke ribbon.
std::vector<QPointF> outwardNormals(const QList<QPointF>& ring) {
    const int n = static_cast<int>(ring.size());
    std::vector<QPointF> normals(static_cast<size_t>(n));
    QPointF centroid(0.0, 0.0);
    for (const QPointF& p : ring) {
        centroid += p;
    }
    centroid /= static_cast<double>(n);

    const auto unit = [](QPointF p) {
        const double l = std::hypot(p.x(), p.y());
        return l > 1e-9 ? QPointF(p.x() / l, p.y() / l) : QPointF(0.0, 0.0);
    };
    const auto leftNormal = [](QPointF e) { return QPointF(-e.y(), e.x()); };

    for (int i = 0; i < n; ++i) {
        const QPointF& prev = ring[(i - 1 + n) % n];
        const QPointF& cur = ring[i];
        const QPointF& next = ring[(i + 1) % n];
        QPointF bisector =
            unit(unit(leftNormal(cur - prev)) + unit(leftNormal(next - cur)));
        if (bisector.isNull()) {
            bisector = unit(leftNormal(next - prev));
        }
        const QPointF outward = cur - centroid;
        if (bisector.x() * outward.x() + bisector.y() * outward.y() < 0.0) {
            bisector = QPointF(-bisector.x(), -bisector.y());
        }
        normals[static_cast<size_t>(i)] = bisector;
    }
    return normals;
}

SmoothVertex vertex(const QPointF& p, uchar r, uchar g, uchar b, uchar a,
    const QPointF& dir = QPointF(0.0, 0.0)) {
    SmoothVertex v;
    v.set(static_cast<float>(p.x()), static_cast<float>(p.y()), r, g, b, a,
        static_cast<float>(dir.x()), static_cast<float>(dir.y()));
    return v;
}

// Builds the fill mesh: an ear-clipped interior at full (premultiplied) colour
// plus an outward feather ring. The feather's outer vertices sit at the same
// local position as the inner ones but carry an outward direction; the shader
// pushes them out by ~1 device pixel and the transparent colour interpolates to
// produce the analytic edge antialiasing.
void buildFillVertices(const QList<QPointF>& ring, const QColor& color,
    std::vector<SmoothVertex>& out) {
    const int n = static_cast<int>(ring.size());
    if (n < 3) {
        return;
    }
    const int alpha = color.alpha();
    const auto pm = [&](int c) { return static_cast<uchar>(c * alpha / 255); };
    const uchar pr = pm(color.red());
    const uchar pg = pm(color.green());
    const uchar pb = pm(color.blue());
    const uchar pa = static_cast<uchar>(alpha);

    std::vector<int> triangles;
    triangulate(ring, triangles);
    const std::vector<QPointF> normals = outwardNormals(ring);
    out.reserve(triangles.size() + static_cast<size_t>(n) * 6);

    for (int idx : triangles) {
        out.push_back(vertex(ring[idx], pr, pg, pb, pa));
    }
    for (int i = 0; i < n; ++i) {
        const int j = (i + 1) % n;
        const QPointF& ni = normals[static_cast<size_t>(i)];
        const QPointF& nj = normals[static_cast<size_t>(j)];
        const SmoothVertex innerI = vertex(ring[i], pr, pg, pb, pa);
        const SmoothVertex innerJ = vertex(ring[j], pr, pg, pb, pa);
        const SmoothVertex outerI = vertex(ring[i], 0, 0, 0, 0, ni);
        const SmoothVertex outerJ = vertex(ring[j], 0, 0, 0, 0, nj);
        out.push_back(innerI);
        out.push_back(outerI);
        out.push_back(outerJ);
        out.push_back(innerI);
        out.push_back(outerJ);
        out.push_back(innerJ);
    }
}

// Builds a centered stroke ribbon (core total width = 2 * halfWidth) along the
// contour, premultiplied, with a ~1 device-pixel feather on its inner and outer
// edges (same shader-expanded mechanism as the fill) so the stroke antialiases
// at any scale. The core width itself stays in local units (scales with the
// shape); only the feather is pinned to device pixels.
void buildStrokeVertices(const QList<QPointF>& ring, const QColor& color,
    double halfWidth, std::vector<SmoothVertex>& out) {
    const int n = static_cast<int>(ring.size());
    if (n < 2 || halfWidth <= 0.0) {
        return;
    }
    const int alpha = color.alpha();
    const auto pm = [&](int c) { return static_cast<uchar>(c * alpha / 255); };
    const uchar pr = pm(color.red());
    const uchar pg = pm(color.green());
    const uchar pb = pm(color.blue());
    const uchar pa = static_cast<uchar>(alpha);

    const std::vector<QPointF> normals = outwardNormals(ring);
    const auto quad = [&](const SmoothVertex& a, const SmoothVertex& b,
                          const SmoothVertex& c, const SmoothVertex& d) {
        out.push_back(a);
        out.push_back(b);
        out.push_back(c);
        out.push_back(a);
        out.push_back(c);
        out.push_back(d);
    };
    out.reserve(static_cast<size_t>(n) * 18);
    for (int i = 0; i < n; ++i) {
        const int j = (i + 1) % n;
        const QPointF& ni = normals[static_cast<size_t>(i)];
        const QPointF& nj = normals[static_cast<size_t>(j)];
        const QPointF inI = ring[i] - ni * halfWidth;
        const QPointF outI = ring[i] + ni * halfWidth;
        const QPointF inJ = ring[j] - nj * halfWidth;
        const QPointF outJ = ring[j] + nj * halfWidth;

        // Core ribbon (full colour).
        const SmoothVertex cInI = vertex(inI, pr, pg, pb, pa);
        const SmoothVertex cOutI = vertex(outI, pr, pg, pb, pa);
        const SmoothVertex cInJ = vertex(inJ, pr, pg, pb, pa);
        const SmoothVertex cOutJ = vertex(outJ, pr, pg, pb, pa);
        quad(cInI, cOutI, cOutJ, cInJ);

        // Outer feather: full colour -> transparent, pushed +1px along normal.
        quad(cOutI, vertex(outI, 0, 0, 0, 0, ni),
            vertex(outJ, 0, 0, 0, 0, nj), cOutJ);

        // Inner feather: full colour -> transparent, pushed -1px along normal.
        quad(cInI, vertex(inI, 0, 0, 0, 0, QPointF(-ni.x(), -ni.y())),
            vertex(inJ, 0, 0, 0, 0, QPointF(-nj.x(), -nj.y())), cInJ);
    }
}

QSGGeometryNode* makeShapeNode() {
    auto* geometry = new QSGGeometry(smoothShapeAttributes(), 0);
    geometry->setDrawingMode(QSGGeometry::DrawTriangles);
    auto* node = new QSGGeometryNode();
    node->setGeometry(geometry);
    node->setMaterial(new SmoothShapeMaterial());
    node->setFlags(QSGNode::OwnsGeometry | QSGNode::OwnsMaterial);
    return node;
}

void uploadVertices(
    QSGGeometryNode* node, const std::vector<SmoothVertex>& verts) {
    QSGGeometry* geometry = node->geometry();
    const int count = static_cast<int>(verts.size());
    if (geometry->vertexCount() != count) {
        geometry->allocate(count);
    }
    if (count > 0) {
        std::memcpy(geometry->vertexData(), verts.data(),
            verts.size() * sizeof(SmoothVertex));
    }
    node->markDirty(QSGNode::DirtyGeometry);
}

// Root node holding the fill mesh and (optionally) the stroke ribbon.
class ShapeNode : public QSGNode {
public:
    QSGGeometryNode* fill = nullptr;
    QSGGeometryNode* stroke = nullptr;
};

} // namespace

QSGNode* MaterialShapeItem::updatePaintNode(
    QSGNode* oldNode, UpdatePaintNodeData*) {
    auto* root = static_cast<ShapeNode*>(oldNode);

    if (width() <= 0.0 || height() <= 0.0 || m_morph == nullptr) {
        delete root;
        return nullptr;
    }

    if (root != nullptr && !m_geometryDirty) {
        return root;
    }

    const std::vector<Cubic> cubics = m_morph->asCubics(m_morphProgress);
    const QList<QPointF> ring = buildRing(cubics, width(), height());
    if (ring.size() < 3) {
        delete root;
        return nullptr;
    }

    if (root == nullptr) {
        root = new ShapeNode();
    }

    // Fill: keep it first so the stroke draws on top.
    if (root->fill == nullptr) {
        root->fill = makeShapeNode();
        root->prependChildNode(root->fill);
    }
    std::vector<SmoothVertex> fillVerts;
    buildFillVertices(ring, m_color, fillVerts);
    uploadVertices(root->fill, fillVerts);

    // Stroke: optional centered ribbon over the same contour.
    const bool wantStroke = m_strokeWidth > 0.0f && m_strokeColor.alpha() > 0;
    if (wantStroke) {
        if (root->stroke == nullptr) {
            root->stroke = makeShapeNode();
            root->appendChildNode(root->stroke);
        }
        std::vector<SmoothVertex> strokeVerts;
        buildStrokeVertices(ring, m_strokeColor,
            static_cast<double>(m_strokeWidth) / 2.0, strokeVerts);
        uploadVertices(root->stroke, strokeVerts);
    } else if (root->stroke != nullptr) {
        root->removeChildNode(root->stroke);
        delete root->stroke;
        root->stroke = nullptr;
    }

    m_geometryDirty = false;
    return root;
}
