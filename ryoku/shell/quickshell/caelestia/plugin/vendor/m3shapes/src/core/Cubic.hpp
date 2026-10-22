#pragma once

#define CUBIC_H

#include "Point.hpp"
#include "Utils.hpp"
#include <array>
#include <utility>

namespace RoundedPolygon {

/**
 * Cubic represents a cubic Bezier curve with two anchor points and two control
 * points. The curve is defined by 8 floats: anchor0(x,y), control0(x,y),
 * control1(x,y), anchor1(x,y)
 */
class Cubic {
public:
    Cubic();
    Cubic(const std::array<float, 8>& points);
    Cubic(float anchor0X, float anchor0Y, float control0X, float control0Y,
        float control1X, float control1Y, float anchor1X, float anchor1Y);
    Cubic(const Point& anchor0, const Point& control0, const Point& control1,
        const Point& anchor1);

    // Accessors
    [[nodiscard]] float anchor0X() const { return m_points[0]; }

    [[nodiscard]] float anchor0Y() const { return m_points[1]; }

    [[nodiscard]] float control0X() const { return m_points[2]; }

    [[nodiscard]] float control0Y() const { return m_points[3]; }

    [[nodiscard]] float control1X() const { return m_points[4]; }

    [[nodiscard]] float control1Y() const { return m_points[5]; }

    [[nodiscard]] float anchor1X() const { return m_points[6]; }

    [[nodiscard]] float anchor1Y() const { return m_points[7]; }

    [[nodiscard]] const std::array<float, 8>& points() const {
        return m_points;
    }

    std::array<float, 8>& points() { return m_points; }

    // Get a point on the curve at parameter t [0,1]
    [[nodiscard]] Point pointOnCurve(float t) const;

    // Check if this cubic is effectively zero-length
    [[nodiscard]] bool zeroLength() const;

    // Check if this curve followed by next forms a convex corner
    [[nodiscard]] bool convexTo(const Cubic& next) const;

    // Calculate axis-aligned bounding box
    // bounds[0]=left, bounds[1]=top, bounds[2]=right, bounds[3]=bottom
    void calculateBounds(
        std::array<float, 4>& bounds, bool approximate = true) const;

    // Split the curve at parameter t, returning two new cubics
    [[nodiscard]] std::pair<Cubic, Cubic> split(float t) const;

    // Reverse the curve direction
    [[nodiscard]] Cubic reverse() const;

    // Transform this cubic using a point transformer
    [[nodiscard]] Cubic transformed(const PointTransformer& f) const;

    // Operators
    [[nodiscard]] Cubic operator+(const Cubic& other) const;
    [[nodiscard]] Cubic operator*(float scalar) const;
    [[nodiscard]] Cubic operator/(float scalar) const;

    [[nodiscard]] bool operator==(const Cubic& other) const;
    [[nodiscard]] bool operator!=(const Cubic& other) const;

    // Static factory methods
    [[nodiscard]] static Cubic straightLine(
        float x0, float y0, float x1, float y1);
    [[nodiscard]] static Cubic circularArc(
        float centerX, float centerY, float x0, float y0, float x1, float y1);
    [[nodiscard]] static Cubic empty(float x0, float y0);

protected:
    std::array<float, 8> m_points;
};

/**
 * MutableCubic is a version of Cubic that allows in-place modifications.
 * Used for performance-critical paths to avoid allocations.
 */
class MutableCubic : public Cubic {
public:
    MutableCubic()
        : Cubic() {}

    // Transform this cubic in place
    void transform(const PointTransformer& f);

    // Interpolate between two cubics, storing result in this
    void interpolate(const Cubic& c1, const Cubic& c2, float progress);
};

} // namespace RoundedPolygon
