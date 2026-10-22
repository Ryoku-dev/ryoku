#pragma once

#include "FloatMapping.hpp"
#include "PolygonMeasure.hpp"
#include <vector>

namespace RoundedPolygon {

/**
 * Creates a DoubleMapper that maps between features of two shapes.
 * This is used to determine how to match curves between shapes for morphing.
 */
[[nodiscard]] DoubleMapper featureMapper(
    const std::vector<ProgressableFeature>& features1,
    const std::vector<ProgressableFeature>& features2);

/**
 * Returns the squared distance between two features.
 * Returns MAX_VALUE if features cannot be mapped (e.g., convex to concave).
 */
[[nodiscard]] float featureDistSquared(const Feature* f1, const Feature* f2);

/**
 * Returns a representative point for a feature.
 */
[[nodiscard]] Point featureRepresentativePoint(const Feature* feature);

} // namespace RoundedPolygon
