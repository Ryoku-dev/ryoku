#pragma once

#include "../core/Utils.hpp"
#include <vector>

namespace RoundedPolygon {

/**
 * Checks if the given progress is in the given progress range. Since progress
 * is in the [0..1) interval and wraps, there is a special case when
 * progressTo < progressFrom. For example, if the progress range is 0.7 to 0.2,
 * both 0.8 and 0.1 are inside and 0.5 is outside.
 */
[[nodiscard]] bool progressInRange(
    float progress, float progressFrom, float progressTo);

/**
 * Distance between two progress values. Since progress wraps around,
 * we consider a difference of 0.99 as a distance of 0.01.
 */
[[nodiscard]] float progressDistance(float p1, float p2);

/**
 * Validates that a list of progress values are all in range [0.0, 1.0)
 * and monotonically increasing, with exception of maybe one wrap-around.
 */
void validateProgress(const std::vector<float>& progress);

/**
 * Maps from one set of progress values to another using linear interpolation.
 */
[[nodiscard]] float linearMap(const std::vector<float>& xValues,
    const std::vector<float>& yValues, float x);

/**
 * DoubleMapper creates mappings from values in the [0..1) source space
 * to values in the [0..1) target space, and back. This mapping is created
 * given a finite list of representative mappings, extended to the whole
 * interval by linear interpolation and wrapping around.
 *
 * This is used to create mappings of progress values between the start
 * and end shape, which is then used to insert new curves and match curves.
 */
class DoubleMapper {
public:
    /**
     * Creates a mapper from a list of source->target mappings.
     */
    DoubleMapper(const std::vector<std::pair<float, float>>& mappings);

    /**
     * Map a value from source space to target space.
     */
    [[nodiscard]] float map(float x) const;

    /**
     * Map a value from target space back to source space.
     */
    [[nodiscard]] float mapBack(float x) const;

    /**
     * Identity mapper (maps x to x).
     */
    static const DoubleMapper Identity;

private:
    std::vector<float> m_sourceValues;
    std::vector<float> m_targetValues;
};

} // namespace RoundedPolygon
