#pragma once

#define CORNERROUNDING_H

#include <cmath>

namespace RoundedPolygon {

/**
 * CornerRounding defines the rounding applied to a polygon vertex.
 *
 * @param radius The radius of the circular arc at the corner. A value of 0
 * means the corner is sharp (unrounded).
 * @param smoothing The amount of smoothing applied to the transition from the
 *                  circular arc to the straight edges. Range is [0, 1] where 0
 *                  means no smoothing (pure circular arc) and 1 means maximum
 *                  smoothing (flanking curves meet at the middle).
 */
struct CornerRounding {
    float radius = 0.0f;
    float smoothing = 0.0f;

    constexpr CornerRounding() = default;

    constexpr CornerRounding(float radius, float smoothing = 0.0f)
        : radius(radius)
        , smoothing(smoothing) {}

    [[nodiscard]] bool operator==(const CornerRounding& other) const {
        constexpr float epsilon = 1e-6f;
        return std::abs(radius - other.radius) < epsilon &&
               std::abs(smoothing - other.smoothing) < epsilon;
    }

    [[nodiscard]] bool operator!=(const CornerRounding& other) const {
        return !(*this == other);
    }

    // Predefined unrounded corner
    static const CornerRounding Unrounded;
};

// Static definition
inline const CornerRounding CornerRounding::Unrounded =
    CornerRounding(0.0f, 0.0f);

} // namespace RoundedPolygon
