#pragma once

#include <QSGGeometry>
#include <QSGMaterial>

/**
 * Vertex format for the analytically-antialiased shape mesh.
 *
 * Layout (tightly packed, 20 bytes):
 *   - x, y        : item-local position
 *   - r, g, b, a  : premultiplied colour (normalized from ubyte in the shader)
 *   - dx, dy      : outward feather DIRECTION in item-local space. (0, 0) marks
 *                   an interior/solid vertex; a unit vector marks a feather
 *                   vertex that the vertex shader pushes outward by a fixed
 *                   device-pixel amount.
 */
struct SmoothVertex {
    float x;
    float y;
    unsigned char r;
    unsigned char g;
    unsigned char b;
    unsigned char a;
    float dx;
    float dy;

    void set(float px, float py, unsigned char cr, unsigned char cg,
        unsigned char cb, unsigned char ca, float ndx, float ndy) {
        x = px;
        y = py;
        r = cr;
        g = cg;
        b = cb;
        a = ca;
        dx = ndx;
        dy = ndy;
    }
};

static_assert(
    sizeof(SmoothVertex) == 20, "SmoothVertex must be tightly packed (20 bytes)");

/** Attribute set describing SmoothVertex to the scene graph. */
const QSGGeometry::AttributeSet& smoothShapeAttributes();

/**
 * Material that draws SmoothVertex geometry with a shader-computed, device-pixel
 * antialiased edge. The feather width is constant in screen pixels regardless of
 * the item's scale, any accumulated parent scale, or the window device pixel
 * ratio, because the expansion happens after the combined matrix in the vertex
 * shader (see smoothshape.vert). Colour is per-vertex, so the material itself is
 * stateless.
 */
class SmoothShapeMaterial : public QSGMaterial {
public:
    SmoothShapeMaterial();

    [[nodiscard]] QSGMaterialType* type() const override;
    [[nodiscard]] QSGMaterialShader* createShader(
        QSGRendererInterface::RenderMode mode) const override;
    [[nodiscard]] int compare(const QSGMaterial* other) const override;
};
