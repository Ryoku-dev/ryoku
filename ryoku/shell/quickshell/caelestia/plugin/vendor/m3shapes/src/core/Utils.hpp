#pragma once

#define UTILS_H

#include "Point.hpp"
#include <algorithm>
#include <cmath>
#include <functional>

namespace RoundedPolygon {

// Constants
constexpr float FloatPi = 3.14159265358979323846f;
constexpr float TwoPi = 2.0f * FloatPi;
constexpr float DistanceEpsilon = 1e-4f;
constexpr float AngleEpsilon = 1e-6f;
constexpr float RelaxedDistanceEpsilon = 5e-3f;

// Helper functions
[[nodiscard]] inline float distance(float x, float y) {
    return std::sqrt(x * x + y * y);
}

[[nodiscard]] inline float distanceSquared(float x, float y) {
    return x * x + y * y;
}

[[nodiscard]] inline float square(float x) {
    return x * x;
}

// Linear interpolation
[[nodiscard]] inline float interpolate(
    float start, float stop, float fraction) {
    return (1.0f - fraction) * start + fraction * stop;
}

// Direction vector from angle
[[nodiscard]] inline Point directionVector(float angleRadians) {
    return Point(std::cos(angleRadians), std::sin(angleRadians));
}

// Direction vector from x,y
[[nodiscard]] inline Point directionVector(float x, float y) {
    float d = distance(x, y);
    if (d <= 0.0f) {
        return Point(0.0f, 0.0f);
    }
    return Point(x / d, y / d);
}

// Convert radial coordinates to cartesian
[[nodiscard]] inline Point radialToCartesian(
    float radius, float angleRadians, const Point& center = Point(0, 0)) {
    return directionVector(angleRadians) * radius + center;
}

// Positive modulo (result is always positive)
[[nodiscard]] inline float positiveModulo(float num, float mod) {
    float result = std::fmod(num, mod);
    if (result < 0) {
        result += mod;
    }
    return result;
}

// Check if three points form a convex corner
[[nodiscard]] inline bool convex(
    const Point& previous, const Point& current, const Point& next) {
    return (current - previous).clockwise(next - current);
}

// Check if C is roughly collinear with AB
[[nodiscard]] inline bool collinearIsh(float aX, float aY, float bX, float bY,
    float cX, float cY, float tolerance = DistanceEpsilon) {
    Point ab = Point(bX - aX, bY - aY).rotate90();
    Point ac = Point(cX - aX, cY - aY);
    float dotProduct = std::abs(ab.dotProduct(ac));
    float relativeTolerance = tolerance * ab.getDistance() * ac.getDistance();
    return dotProduct < tolerance || dotProduct < relativeTolerance;
}

// Transform result pair
struct TransformResult {
    float x;
    float y;

    TransformResult(float x, float y)
        : x(x)
        , y(y) {}
};

// Point transformer function type
using PointTransformer = std::function<TransformResult(float x, float y)>;

// Transform a point
[[nodiscard]] inline Point transformed(
    const Point& p, const PointTransformer& f) {
    auto result = f(p.x, p.y);
    return Point(result.x, result.y);
}

} // namespace RoundedPolygon
