#pragma once

#include "../morph/Morph.hpp"
#include "../shapes/MaterialShapes.hpp"
#include <QEasingCurve>
#include <QPainterPath>
#include <QPropertyAnimation>
#include <QQuickItem>
#include <QVariantList>
#include <memory>
#include <optional>

namespace RoundedPolygon {

class RoundedPolygonShape;

}

/**
 * QML wrapper for RoundedPolygonShape.
 * Used to pass custom shapes between QML and C++.
 */
class RoundedPolygonWrapper {
    Q_GADGET
    QML_VALUE_TYPE(roundedPolygon)
    Q_PROPERTY(bool valid READ isValid FINAL)

public:
    RoundedPolygonWrapper() = default;
    explicit RoundedPolygonWrapper(
        const RoundedPolygon::RoundedPolygonShape& shape);

    [[nodiscard]] bool isValid() const { return m_shape.has_value(); }

    [[nodiscard]] const RoundedPolygon::RoundedPolygonShape& shape() const;

    Q_INVOKABLE RoundedPolygonWrapper normalized() const;

private:
    std::optional<RoundedPolygon::RoundedPolygonShape> m_shape;
};

/**
 * MaterialShapeItem is a unified QML component for Material Design shapes.
 *
 * Predefined shapes with auto-morph:
 *   MaterialShape {
 *       shape: MaterialShape.Heart
 *       animationDuration: 1000
 *       animationEasing: Easing.OutBounce
 *   }
 *
 * Custom shapes using factory functions (shape must be set to Custom):
 *   MaterialShape {
 *       shape: MaterialShape.Custom
 *       customShape: MaterialShape.polygon([
 *           MaterialShape.point(0.5, 0, 0.2, 0.5),
 *           MaterialShape.point(1, 0.5, 0.2, 0.5),
 *           MaterialShape.point(0.5, 1, 0.2, 0.5),
 *           MaterialShape.point(0, 0.5, 0.2, 0.5)
 *       ])
 *   }
 *
 * Or with object syntax:
 *   MaterialShape {
 *       shape: MaterialShape.Custom
 *       customShape: MaterialShape.polygon([
 *           { x: 0.5, y: 0, radius: 0.2, smoothing: 0.5 },
 *           { x: 1, y: 0.5 }  // uses default radius/smoothing of 0
 *       ])
 *   }
 *
 * Manual morphing between custom and predefined shapes:
 *   MaterialShape {
 *       customFromShape: MaterialShape.polygon([...])
 *       toShape: MaterialShape.Heart
 *       morphProgress: slider.value
 *   }
 */
class MaterialShapeItem : public QQuickItem {
    Q_OBJECT
    QML_ELEMENT
    QML_NAMED_ELEMENT(MaterialShape)

public:
    enum Shape {
        Custom = -1,
        Circle = 0,
        Square,
        Slanted,
        Arch,
        Fan,
        Arrow,
        SemiCircle,
        Oval,
        Pill,
        Triangle,
        Diamond,
        ClamShell,
        Pentagon,
        Gem,
        Sunny,
        VerySunny,
        Cookie4Sided,
        Cookie6Sided,
        Cookie7Sided,
        Cookie9Sided,
        Cookie12Sided,
        Ghostish,
        Clover4Leaf,
        Clover8Leaf,
        Burst,
        SoftBurst,
        Boom,
        SoftBoom,
        Flower,
        Puffy,
        PuffyDiamond,
        PixelCircle,
        PixelTriangle,
        Bun,
        Heart
    };
    Q_ENUM(Shape)

    Q_PROPERTY(Shape shape READ shape WRITE setShape NOTIFY shapeChanged)
    Q_PROPERTY(Shape fromShape READ fromShape WRITE setFromShape NOTIFY
            fromShapeChanged)
    Q_PROPERTY(
        Shape toShape READ toShape WRITE setToShape NOTIFY toShapeChanged)
    Q_PROPERTY(int animationDuration READ animationDuration WRITE
            setAnimationDuration NOTIFY animationDurationChanged)
    Q_PROPERTY(QEasingCurve animationEasing READ animationEasing WRITE
            setAnimationEasing NOTIFY animationEasingChanged)
    Q_PROPERTY(QColor color READ color WRITE setColor NOTIFY colorChanged)
    Q_PROPERTY(qreal implicitSize READ implicitSize WRITE setImplicitSize NOTIFY
            implicitSizeChanged)
    Q_PROPERTY(QColor strokeColor READ strokeColor WRITE setStrokeColor NOTIFY
            strokeColorChanged)
    Q_PROPERTY(float strokeWidth READ strokeWidth WRITE setStrokeWidth NOTIFY
            strokeWidthChanged)
    Q_PROPERTY(float morphProgress READ morphProgress WRITE setMorphProgress
            NOTIFY morphProgressChanged)
    Q_PROPERTY(RoundedPolygonWrapper customShape READ customShape WRITE
            setCustomShape NOTIFY customShapeChanged)
    Q_PROPERTY(RoundedPolygonWrapper customFromShape READ customFromShape WRITE
            setCustomFromShape NOTIFY customFromShapeChanged)
    Q_PROPERTY(RoundedPolygonWrapper customToShape READ customToShape WRITE
            setCustomToShape NOTIFY customToShapeChanged)

    explicit MaterialShapeItem(QQuickItem* parent = nullptr);

    // ========== Factory functions for custom shapes ==========

    /**
     * Create a point with optional rounding for use in polygon().
     * @param x X coordinate (0-1 normalized)
     * @param y Y coordinate (0-1 normalized)
     * @param radius Corner radius (default 0)
     * @param smoothing Corner smoothing 0-1 (default 0)
     */
    Q_INVOKABLE static QVariantMap point(
        float x, float y, float radius = 0.0f, float smoothing = 0.0f);

    /**
     * Create a custom polygon from vertices.
     * @param vertices Array of points (from point() or {x, y, radius,
     * smoothing})
     * @param reps Number of rotational repetitions (default 1)
     * @param centerX Center X coordinate (default 0.5)
     * @param centerY Center Y coordinate (default 0.5)
     * @param mirroring Enable mirror symmetry (default false)
     */
    Q_INVOKABLE static RoundedPolygonWrapper polygon(
        const QVariantList& vertices, int reps = 1, float centerX = 0.5f,
        float centerY = 0.5f, bool mirroring = false);

    /**
     * Create a regular polygon with N vertices.
     * @param numVertices Number of vertices (minimum 3)
     * @param radius Corner radius (default 0)
     * @param smoothing Corner smoothing 0-1 (default 0)
     */
    Q_INVOKABLE static RoundedPolygonWrapper regularPolygon(
        int numVertices, float radius = 0.0f, float smoothing = 0.0f);

    /**
     * Create a star shape.
     * @param points Number of star points
     * @param innerRadius Ratio of inner to outer radius (0-1)
     * @param radius Corner radius (default 0)
     * @param smoothing Corner smoothing (default 0)
     */
    Q_INVOKABLE static RoundedPolygonWrapper star(int points,
        float innerRadius = 0.5f, float radius = 0.0f, float smoothing = 0.0f);

    /**
     * Create a rectangle shape.
     * @param width Width (default 1)
     * @param height Height (default 1)
     * @param radius Corner radius (default 0)
     * @param smoothing Corner smoothing (default 0)
     */
    Q_INVOKABLE static RoundedPolygonWrapper rectangle(float width = 1.0f,
        float height = 1.0f, float radius = 0.0f, float smoothing = 0.0f);

    /**
     * Create a squircle (superellipse) shape.
     * Uses the equation |x|^n + |y|^n = 1
     * @param n Exponent controlling squareness (2=ellipse, 4=squircle,
     * higher=more square)
     * @param segments Number of points to generate (default 64,
     * higher=smoother)
     */
    Q_INVOKABLE static RoundedPolygonWrapper squircle(
        float n = 4.0f, int segments = 64);

    // ========== Path queries ==========

    /**
     * Get the point on the shape's edge along a ray from the center.
     * Convention: 0° = up (12 o'clock), positive = clockwise (matches
     * Item.rotation). The angle is interpreted in the item's parent frame —
     * the item's own rotation is taken into account, so 0° always points
     * "up" on screen regardless of how the shape is rotated. For non-convex
     * shapes the OUTERMOST intersection is returned.
     * @param angleDegrees Direction of the ray in degrees, parent-frame.
     * @return Point in item-local (pre-rotation) coordinates — place a child
     *         of this item there and the item's transform will move it onto
     *         the visible edge automatically. Returns (0, 0) if empty.
     */
    Q_INVOKABLE QPointF pointAtAngle(qreal angleDegrees) const;

    /**
     * Distance from the shape's center to its edge along a ray at the given
     * angle. Same conventions as pointAtAngle.
     * @return Distance in pixels, or 0 if the path is empty / no hit.
     */
    Q_INVOKABLE qreal distanceAtAngle(qreal angleDegrees) const;

    /**
     * Bounding rectangle of the current shape, in item-local coordinates.
     */
    Q_INVOKABLE QRectF pathBounds() const;

    /**
     * Defer morph rebuilds until the matching endBatchUpdate(). Useful when
     * setting several morph-related properties (fromShape, toShape, custom*)
     * at once so the underlying Morph is constructed only one time. Calls
     * nest — every begin must be paired with an end.
     */
    Q_INVOKABLE void beginBatchUpdate();

    /**
     * End a batch started by beginBatchUpdate(). When the outermost batch
     * closes, performs a single rebuild if any property changed during the
     * batch.
     */
    Q_INVOKABLE void endBatchUpdate();

    // ========== Property accessors ==========

    [[nodiscard]] Shape shape() const { return m_targetShape; }

    void setShape(Shape shape);

    [[nodiscard]] Shape fromShape() const { return m_fromShape; }

    void setFromShape(Shape shape);

    [[nodiscard]] Shape toShape() const { return m_toShape; }

    void setToShape(Shape shape);

    [[nodiscard]] int animationDuration() const { return m_animationDuration; }

    void setAnimationDuration(int duration);

    [[nodiscard]] QEasingCurve animationEasing() const {
        return m_animationEasing;
    }

    void setAnimationEasing(const QEasingCurve& easing);

    [[nodiscard]] QColor color() const { return m_color; }

    void setColor(const QColor& color);

    [[nodiscard]] qreal implicitSize() const { return m_implicitSize; }

    void setImplicitSize(qreal size);

    [[nodiscard]] QColor strokeColor() const { return m_strokeColor; }

    void setStrokeColor(const QColor& color);

    [[nodiscard]] float strokeWidth() const { return m_strokeWidth; }

    void setStrokeWidth(float width);

    [[nodiscard]] float morphProgress() const { return m_morphProgress; }

    void setMorphProgress(float progress);

    [[nodiscard]] RoundedPolygonWrapper customShape() const {
        return m_customShape;
    }

    void setCustomShape(const RoundedPolygonWrapper& shape);

    [[nodiscard]] RoundedPolygonWrapper customFromShape() const {
        return m_customFromShape;
    }

    void setCustomFromShape(const RoundedPolygonWrapper& shape);

    [[nodiscard]] RoundedPolygonWrapper customToShape() const {
        return m_customToShape;
    }

    void setCustomToShape(const RoundedPolygonWrapper& shape);

    bool contains(const QPointF& point) const override;

signals:
    void shapeChanged();
    void fromShapeChanged();
    void toShapeChanged();
    void animationDurationChanged();
    void animationEasingChanged();
    void colorChanged();
    void implicitSizeChanged();
    void strokeColorChanged();
    void strokeWidthChanged();
    void morphProgressChanged();
    void customShapeChanged();
    void customFromShapeChanged();
    void customToShapeChanged();

protected:
    QSGNode* updatePaintNode(
        QSGNode* oldNode, UpdatePaintNodeData* data) override;

private slots:
    void onAnimationValueChanged(const QVariant& value);
    void onMorphFinished();

protected:
    void geometryChange(
        const QRectF& newGeometry, const QRectF& oldGeometry) override;

private:
    QPainterPath buildPath() const;
    const QPainterPath& cachedPath() const;
    const QList<QPolygonF>& cachedPolygons() const;
    qreal rayHitDistance(qreal dx, qreal dy) const;
    void invalidatePath();
    void startMorph(Shape from, Shape to);
    void rebuildMorph();
    RoundedPolygon::RoundedPolygonShape getShapeForEnum(Shape shape) const;

    Shape m_currentShape = Circle;
    Shape m_targetShape = Circle;
    Shape m_fromShape = Circle;
    Shape m_toShape = Circle;
    int m_animationDuration = 350;
    QEasingCurve m_animationEasing;
    float m_morphProgress = 1.0f;
    QColor m_color = Qt::black;
    qreal m_implicitSize = 0.0;
    QColor m_strokeColor = Qt::transparent;
    float m_strokeWidth = 0.0f;

    std::unique_ptr<RoundedPolygon::Morph> m_morph;
    QPropertyAnimation* m_animation = nullptr;

    mutable QPainterPath m_cachedPath;
    mutable QList<QPolygonF> m_cachedPolygons;
    mutable bool m_pathDirty = true;
    mutable bool m_polygonsDirty = true;
    bool m_geometryDirty = true;

    RoundedPolygonWrapper m_customShape;
    RoundedPolygonWrapper m_customFromShape;
    RoundedPolygonWrapper m_customToShape;

    int m_batchDepth = 0;
    bool m_pendingRebuild = false;
};
