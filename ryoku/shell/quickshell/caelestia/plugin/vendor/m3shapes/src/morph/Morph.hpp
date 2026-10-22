#pragma once

#include "../core/RoundedPolygon.hpp"
#include "FeatureMapping.hpp"
#include "PolygonMeasure.hpp"
#include <functional>
#include <vector>

namespace RoundedPolygon {

/**
 * Morph is used to animate between start and end polygon objects.
 *
 * Morphing between arbitrary objects can be problematic because it can be
 * difficult to determine how the points of a given shape map to the points
 * of some other shape. Morph simplifies the problem by only operating on
 * RoundedPolygonShape objects, which have similar, contiguous structures.
 *
 * The morph works by determining how to map the curves of the two shapes
 * together (based on proximity and other information, such as distance to
 * polygon vertices and concavity), and splitting curves when the shapes
 * do not have the same number of curves.
 */
class Morph {
public:
    /**
     * Create a morph between two shapes.
     */
    Morph(const RoundedPolygonShape& start, const RoundedPolygonShape& end);

    /**
     * Returns a representation of the morph at a given progress value
     * as a list of Cubics.
     *
     * @param progress Value from 0 to 1. A value of 0 results in the start
     *        shape, 1 results in the end shape, and values in between are
     *        linear interpolations between those shapes.
     */
    [[nodiscard]] std::vector<Cubic> asCubics(float progress) const;

    /**
     * Iterates over cubics at the given progress, calling the callback
     * for each one. More efficient than asCubics() as it reuses a
     * MutableCubic instance.
     *
     * @param progress Value from 0 to 1 determining the morph state.
     * @param callback Function called for each cubic.
     */
    void forEachCubic(float progress,
        const std::function<void(const MutableCubic&)>& callback) const;

    /**
     * Calculate the axis-aligned bounding box of the morph.
     *
     * @param approximate When true, uses faster calculation based on
     *        min/max of anchor and control points.
     */
    [[nodiscard]] std::array<float, 4> calculateBounds(
        bool approximate = true) const;

    /**
     * Calculate the maximum bounding box that can hold the shape
     * in any rotation.
     */
    [[nodiscard]] std::array<float, 4> calculateMaxBounds() const;

    /**
     * Get the matched cubic pairs (for debugging/visualization).
     */
    [[nodiscard]] const std::vector<std::pair<Cubic, Cubic>>&
    morphMatch() const {
        return m_morphMatch;
    }

private:
    RoundedPolygonShape m_start;
    RoundedPolygonShape m_end;
    std::vector<std::pair<Cubic, Cubic>> m_morphMatch;

    /**
     * Match features between two shapes, creating paired cubics
     * that can be interpolated.
     */
    static std::vector<std::pair<Cubic, Cubic>> match(
        const RoundedPolygonShape& p1, const RoundedPolygonShape& p2);
};

} // namespace RoundedPolygon
