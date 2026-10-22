#include "Cubic.hpp"
#include <algorithm>
#include <cmath>

namespace RoundedPolygon {

Cubic::Cubic()
    : m_points{ 0, 0, 0, 0, 0, 0, 0, 0 } {}

Cubic::Cubic(const std::array<float, 8>& points)
    : m_points(points) {}

Cubic::Cubic(float anchor0X, float anchor0Y, float control0X, float control0Y,
    float control1X, float control1Y, float anchor1X, float anchor1Y)
    : m_points{ anchor0X, anchor0Y, control0X, control0Y, control1X, control1Y,
        anchor1X, anchor1Y } {}

Cubic::Cubic(const Point& anchor0, const Point& control0, const Point& control1,
    const Point& anchor1)
    : m_points{ anchor0.x, anchor0.y, control0.x, control0.y, control1.x,
        control1.y, anchor1.x, anchor1.y } {}

Point Cubic::pointOnCurve(float t) const {
    float u = 1.0f - t;
    float u2 = u * u;
    float u3 = u2 * u;
    float t2 = t * t;
    float t3 = t2 * t;

    return Point(anchor0X() * u3 + control0X() * 3.0f * t * u2 +
                     control1X() * 3.0f * t2 * u + anchor1X() * t3,
        anchor0Y() * u3 + control0Y() * 3.0f * t * u2 +
            control1Y() * 3.0f * t2 * u + anchor1Y() * t3);
}

bool Cubic::zeroLength() const {
    return std::abs(anchor0X() - anchor1X()) < DistanceEpsilon &&
           std::abs(anchor0Y() - anchor1Y()) < DistanceEpsilon;
}

bool Cubic::convexTo(const Cubic& next) const {
    Point prevVertex(anchor0X(), anchor0Y());
    Point currVertex(anchor1X(), anchor1Y());
    Point nextVertex(next.anchor1X(), next.anchor1Y());
    return convex(prevVertex, currVertex, nextVertex);
}

void Cubic::calculateBounds(
    std::array<float, 4>& bounds, bool approximate) const {
    if (zeroLength()) {
        bounds[0] = anchor0X();
        bounds[1] = anchor0Y();
        bounds[2] = anchor0X();
        bounds[3] = anchor0Y();
        return;
    }

    float minX = std::min(anchor0X(), anchor1X());
    float minY = std::min(anchor0Y(), anchor1Y());
    float maxX = std::max(anchor0X(), anchor1X());
    float maxY = std::max(anchor0Y(), anchor1Y());

    if (approximate) {
        bounds[0] = std::min({ minX, control0X(), control1X() });
        bounds[1] = std::min({ minY, control0Y(), control1Y() });
        bounds[2] = std::max({ maxX, control0X(), control1X() });
        bounds[3] = std::max({ maxY, control0Y(), control1Y() });
        return;
    }

    // Exact bounds calculation using derivative
    auto checkExtreme = [this, &minX, &maxX, &minY, &maxY](float t, bool isX) {
        if (t > 0.0f && t < 1.0f) {
            Point p = pointOnCurve(t);
            if (isX) {
                minX = std::min(minX, p.x);
                maxX = std::max(maxX, p.x);
            } else {
                minY = std::min(minY, p.y);
                maxY = std::max(maxY, p.y);
            }
        }
    };

    // X coordinate extremes
    float xa =
        -anchor0X() + 3.0f * control0X() - 3.0f * control1X() + anchor1X();
    float xb = 2.0f * anchor0X() - 4.0f * control0X() + 2.0f * control1X();
    float xc = -anchor0X() + control0X();

    if (std::abs(xa) < DistanceEpsilon) {
        if (xb != 0.0f) {
            checkExtreme(2.0f * xc / (-2.0f * xb), true);
        }
    } else {
        float xs = xb * xb - 4.0f * xa * xc;
        if (xs >= 0) {
            float sqrtXs = std::sqrt(xs);
            checkExtreme((-xb + sqrtXs) / (2.0f * xa), true);
            checkExtreme((-xb - sqrtXs) / (2.0f * xa), true);
        }
    }

    // Y coordinate extremes
    float ya =
        -anchor0Y() + 3.0f * control0Y() - 3.0f * control1Y() + anchor1Y();
    float yb = 2.0f * anchor0Y() - 4.0f * control0Y() + 2.0f * control1Y();
    float yc = -anchor0Y() + control0Y();

    if (std::abs(ya) < DistanceEpsilon) {
        if (yb != 0.0f) {
            checkExtreme(2.0f * yc / (-2.0f * yb), false);
        }
    } else {
        float ys = yb * yb - 4.0f * ya * yc;
        if (ys >= 0) {
            float sqrtYs = std::sqrt(ys);
            checkExtreme((-yb + sqrtYs) / (2.0f * ya), false);
            checkExtreme((-yb - sqrtYs) / (2.0f * ya), false);
        }
    }

    bounds[0] = minX;
    bounds[1] = minY;
    bounds[2] = maxX;
    bounds[3] = maxY;
}

std::pair<Cubic, Cubic> Cubic::split(float t) const {
    float u = 1.0f - t;
    Point onCurve = pointOnCurve(t);

    Cubic first(anchor0X(), anchor0Y(), anchor0X() * u + control0X() * t,
        anchor0Y() * u + control0Y() * t,
        anchor0X() * u * u + control0X() * 2.0f * u * t + control1X() * t * t,
        anchor0Y() * u * u + control0Y() * 2.0f * u * t + control1Y() * t * t,
        onCurve.x, onCurve.y);

    Cubic second(onCurve.x, onCurve.y,
        control0X() * u * u + control1X() * 2.0f * u * t + anchor1X() * t * t,
        control0Y() * u * u + control1Y() * 2.0f * u * t + anchor1Y() * t * t,
        control1X() * u + anchor1X() * t, control1Y() * u + anchor1Y() * t,
        anchor1X(), anchor1Y());

    return { first, second };
}

Cubic Cubic::reverse() const {
    return Cubic(anchor1X(), anchor1Y(), control1X(), control1Y(), control0X(),
        control0Y(), anchor0X(), anchor0Y());
}

Cubic Cubic::transformed(const PointTransformer& f) const {
    std::array<float, 8> newPoints;
    for (size_t i = 0; i < 4; ++i) {
        auto result = f(m_points[i * 2], m_points[i * 2 + 1]);
        newPoints[i * 2] = result.x;
        newPoints[i * 2 + 1] = result.y;
    }
    return Cubic(newPoints);
}

Cubic Cubic::operator+(const Cubic& other) const {
    std::array<float, 8> result;
    for (size_t i = 0; i < 8; ++i) {
        result[i] = m_points[i] + other.m_points[i];
    }
    return Cubic(result);
}

Cubic Cubic::operator*(float scalar) const {
    std::array<float, 8> result;
    for (size_t i = 0; i < 8; ++i) {
        result[i] = m_points[i] * scalar;
    }
    return Cubic(result);
}

Cubic Cubic::operator/(float scalar) const {
    return *this * (1.0f / scalar);
}

bool Cubic::operator==(const Cubic& other) const {
    return m_points == other.m_points;
}

bool Cubic::operator!=(const Cubic& other) const {
    return !(*this == other);
}

Cubic Cubic::straightLine(float x0, float y0, float x1, float y1) {
    return Cubic(x0, y0, interpolate(x0, x1, 1.0f / 3.0f),
        interpolate(y0, y1, 1.0f / 3.0f), interpolate(x0, x1, 2.0f / 3.0f),
        interpolate(y0, y1, 2.0f / 3.0f), x1, y1);
}

Cubic Cubic::circularArc(
    float centerX, float centerY, float x0, float y0, float x1, float y1) {
    Point p0d = directionVector(x0 - centerX, y0 - centerY);
    Point p1d = directionVector(x1 - centerX, y1 - centerY);
    Point rotatedP0 = p0d.rotate90();
    Point rotatedP1 = p1d.rotate90();

    bool clockwise = rotatedP0.dotProduct(x1 - centerX, y1 - centerY) >= 0;
    float cosa = p0d.dotProduct(p1d);

    // Near-identical points, return straight line
    if (cosa > 0.999f) {
        return straightLine(x0, y0, x1, y1);
    }

    float radius = distance(x0 - centerX, y0 - centerY);
    float k =
        radius * 4.0f / 3.0f *
        (std::sqrt(2.0f * (1.0f - cosa)) - std::sqrt(1.0f - cosa * cosa)) /
        (1.0f - cosa) * (clockwise ? 1.0f : -1.0f);

    return Cubic(x0, y0, x0 + rotatedP0.x * k, y0 + rotatedP0.y * k,
        x1 - rotatedP1.x * k, y1 - rotatedP1.y * k, x1, y1);
}

Cubic Cubic::empty(float x0, float y0) {
    return Cubic(x0, y0, x0, y0, x0, y0, x0, y0);
}

// MutableCubic implementation

void MutableCubic::transform(const PointTransformer& f) {
    for (size_t i = 0; i < 4; ++i) {
        auto result = f(m_points[i * 2], m_points[i * 2 + 1]);
        m_points[i * 2] = result.x;
        m_points[i * 2 + 1] = result.y;
    }
}

void MutableCubic::interpolate(
    const Cubic& c1, const Cubic& c2, float progress) {
    const auto& p1 = c1.points();
    const auto& p2 = c2.points();
    for (size_t i = 0; i < 8; ++i) {
        m_points[i] = RoundedPolygon::interpolate(p1[i], p2[i], progress);
    }
}

} // namespace RoundedPolygon
