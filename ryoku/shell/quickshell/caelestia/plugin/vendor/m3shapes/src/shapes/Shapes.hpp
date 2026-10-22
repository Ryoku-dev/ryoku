#pragma once

#include "core/RoundedPolygon.hpp"

namespace RoundedPolygon {

/**
 * Factory functions for creating common polygon shapes.
 */
class Shapes {
public:
    /**
     * Creates a circular polygon with the given number of vertices.
     *
     * @param numVertices Number of vertices (more = smoother circle)
     * @param radius Radius of the circle
     * @param centerX X coordinate of center
     * @param centerY Y coordinate of center
     */
    [[nodiscard]] static RoundedPolygonShape circle(int numVertices = 8,
        float radius = 1.0f, float centerX = 0.0f, float centerY = 0.0f);

    /**
     * Creates a rectangle with optional corner rounding.
     *
     * @param width Width of rectangle
     * @param height Height of rectangle
     * @param rounding Corner rounding for all corners
     * @param perVertexRounding Per-vertex rounding (optional)
     * @param centerX X coordinate of center
     * @param centerY Y coordinate of center
     */
    [[nodiscard]] static RoundedPolygonShape rectangle(float width = 2.0f,
        float height = 2.0f,
        const CornerRounding& rounding = CornerRounding::Unrounded,
        const std::vector<CornerRounding>* perVertexRounding = nullptr,
        float centerX = 0.0f, float centerY = 0.0f);

    /**
     * Creates a star polygon with inner and outer vertices.
     *
     * @param numVerticesPerRadius Number of points on the star
     * @param radius Outer radius
     * @param innerRadius Inner radius (valley depth)
     * @param rounding Corner rounding for all vertices
     * @param innerRounding Rounding for inner vertices (optional)
     * @param perVertexRounding Per-vertex rounding (optional)
     * @param centerX X coordinate of center
     * @param centerY Y coordinate of center
     */
    [[nodiscard]] static RoundedPolygonShape star(int numVerticesPerRadius,
        float radius = 1.0f, float innerRadius = 0.5f,
        const CornerRounding& rounding = CornerRounding::Unrounded,
        const CornerRounding* innerRounding = nullptr,
        const std::vector<CornerRounding>* perVertexRounding = nullptr,
        float centerX = 0.0f, float centerY = 0.0f);

    /**
     * Creates a pill shape (stadium shape).
     *
     * @param width Width of the pill
     * @param height Height of the pill
     * @param smoothing Smoothing factor for transitions
     * @param centerX X coordinate of center
     * @param centerY Y coordinate of center
     */
    [[nodiscard]] static RoundedPolygonShape pill(float width = 2.0f,
        float height = 1.0f, float smoothing = 0.0f, float centerX = 0.0f,
        float centerY = 0.0f);

    /**
     * Creates a pill-star hybrid shape.
     *
     * @param width Width of the shape
     * @param height Height of the shape
     * @param numVerticesPerRadius Number of points
     * @param innerRadiusRatio Ratio of inner to outer radius
     * @param rounding Corner rounding
     * @param innerRounding Inner vertex rounding
     * @param perVertexRounding Per-vertex rounding
     * @param vertexSpacing Spacing adjustment for vertices
     * @param startLocation Starting rotation
     * @param centerX X coordinate of center
     * @param centerY Y coordinate of center
     */
    [[nodiscard]] static RoundedPolygonShape pillStar(float width = 2.0f,
        float height = 1.0f, int numVerticesPerRadius = 8,
        float innerRadiusRatio = 0.5f,
        const CornerRounding& rounding = CornerRounding::Unrounded,
        const CornerRounding* innerRounding = nullptr,
        const std::vector<CornerRounding>* perVertexRounding = nullptr,
        float vertexSpacing = 0.5f, float startLocation = 0.0f,
        float centerX = 0.0f, float centerY = 0.0f);

private:
    Shapes() = default;
};

} // namespace RoundedPolygon
