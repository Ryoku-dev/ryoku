#pragma once

#define POINT_H

#include <cmath>
#include <utility>

namespace RoundedPolygon {

struct Point {
    float x = 0.0f;
    float y = 0.0f;

    constexpr Point() = default;

    constexpr Point(float x, float y)
        : x(x)
        , y(y) {}

    // Magnitude (distance from origin)
    [[nodiscard]] float getDistance() const { return std::sqrt(x * x + y * y); }

    [[nodiscard]] float getDistanceSquared() const { return x * x + y * y; }

    // Dot product
    [[nodiscard]] float dotProduct(const Point& other) const {
        return x * other.x + y * other.y;
    }

    [[nodiscard]] float dotProduct(float otherX, float otherY) const {
        return x * otherX + y * otherY;
    }

    // Cross product Z component (for clockwise check)
    [[nodiscard]] bool clockwise(const Point& other) const {
        return x * other.y - y * other.x > 0;
    }

    // Unit vector direction
    [[nodiscard]] Point getDirection() const {
        float d = getDistance();
        if (d <= 0.0f) {
            return Point(0.0f, 0.0f);
        }
        return Point(x / d, y / d);
    }

    // Rotate 90 degrees counterclockwise
    [[nodiscard]] Point rotate90() const { return Point(-y, x); }

    // Operators
    [[nodiscard]] Point operator-() const { return Point(-x, -y); }

    [[nodiscard]] Point operator+(const Point& other) const {
        return Point(x + other.x, y + other.y);
    }

    [[nodiscard]] Point operator-(const Point& other) const {
        return Point(x - other.x, y - other.y);
    }

    [[nodiscard]] Point operator*(float scalar) const {
        return Point(x * scalar, y * scalar);
    }

    [[nodiscard]] Point operator/(float scalar) const {
        return Point(x / scalar, y / scalar);
    }

    Point& operator+=(const Point& other) {
        x += other.x;
        y += other.y;
        return *this;
    }

    Point& operator-=(const Point& other) {
        x -= other.x;
        y -= other.y;
        return *this;
    }

    [[nodiscard]] bool operator==(const Point& other) const {
        constexpr float epsilon = 1e-6f;
        return std::abs(x - other.x) < epsilon &&
               std::abs(y - other.y) < epsilon;
    }

    [[nodiscard]] bool operator!=(const Point& other) const {
        return !(*this == other);
    }
};

// Free function for scalar * Point
[[nodiscard]] inline Point operator*(float scalar, const Point& p) {
    return p * scalar;
}

// Linear interpolation between two points
[[nodiscard]] inline Point interpolate(
    const Point& start, const Point& stop, float fraction) {
    return Point((1.0f - fraction) * start.x + fraction * stop.x,
        (1.0f - fraction) * start.y + fraction * stop.y);
}

} // namespace RoundedPolygon
