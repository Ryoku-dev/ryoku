#pragma once

#include "CornerRounding.hpp"
#include "Feature.hpp"
#include <memory>
#include <optional>
#include <vector>

namespace RoundedPolygon {

/**
 * RoundedPolygon allows simple construction of polygonal shapes with optional
 * rounding at the vertices. Polygons can be constructed with either the number
 * of vertices desired or an ordered list of vertices.
 */
class RoundedPolygonShape {
public:
    // Constructor from features (internal use)
    RoundedPolygonShape(
        std::vector<std::unique_ptr<Feature>> features, const Point& center);

    // Constructor from number of vertices (regular polygon)
    RoundedPolygonShape(int numVertices, float radius = 1.0f,
        float centerX = 0.0f, float centerY = 0.0f,
        const CornerRounding& rounding = CornerRounding::Unrounded,
        const std::vector<CornerRounding>* perVertexRounding = nullptr);

    // Constructor from vertex array
    RoundedPolygonShape(const std::vector<float>& vertices,
        const CornerRounding& rounding = CornerRounding::Unrounded,
        const std::vector<CornerRounding>* perVertexRounding = nullptr,
        float centerX = std::numeric_limits<float>::lowest(),
        float centerY = std::numeric_limits<float>::lowest());

    // Copy constructor
    RoundedPolygonShape(const RoundedPolygonShape& other);

    // Move constructor
    RoundedPolygonShape(RoundedPolygonShape&& other) noexcept;

    // Copy assignment
    RoundedPolygonShape& operator=(const RoundedPolygonShape& other);

    // Move assignment
    RoundedPolygonShape& operator=(RoundedPolygonShape&& other) noexcept;

    ~RoundedPolygonShape() = default;

    // Accessors
    [[nodiscard]] float centerX() const { return m_center.x; }

    [[nodiscard]] float centerY() const { return m_center.y; }

    [[nodiscard]] const Point& center() const { return m_center; }

    [[nodiscard]] const std::vector<std::unique_ptr<Feature>>&
    features() const {
        return m_features;
    }

    [[nodiscard]] const std::vector<Cubic>& cubics() const { return m_cubics; }

    // Transform this polygon with a point transformer
    [[nodiscard]] RoundedPolygonShape transformed(
        const PointTransformer& f) const;

    // Normalize polygon to fit within unit square (0,0)-(1,1)
    [[nodiscard]] RoundedPolygonShape normalized() const;

    // Calculate axis-aligned bounding box
    // bounds[0]=left, bounds[1]=top, bounds[2]=right, bounds[3]=bottom
    void calculateBounds(
        std::array<float, 4>& bounds, bool approximate = true) const;
    [[nodiscard]] std::array<float, 4> calculateBounds(
        bool approximate = true) const;

    // Calculate max bounds (square that can hold shape in any rotation)
    void calculateMaxBounds(std::array<float, 4>& bounds) const;
    [[nodiscard]] std::array<float, 4> calculateMaxBounds() const;

private:
    std::vector<std::unique_ptr<Feature>> m_features;
    Point m_center;
    std::vector<Cubic> m_cubics;

    void buildCubics();
    static Point calculateCenterFromVertices(
        const std::vector<float>& vertices);
    static std::vector<float> verticesFromNumVerts(
        int numVertices, float radius, float centerX, float centerY);
};

// Helper class for corner rounding calculations
class RoundedCorner {
public:
    RoundedCorner(const Point& p0, const Point& p1, const Point& p2,
        const CornerRounding& rounding);

    [[nodiscard]] float expectedRoundCut() const { return m_expectedRoundCut; }

    [[nodiscard]] float expectedCut() const {
        return (1.0f + m_smoothing) * m_expectedRoundCut;
    }

    [[nodiscard]] std::vector<Cubic> getCubics(
        float allowedCut0, float allowedCut1) const;

private:
    Point m_p0, m_p1, m_p2;
    Point m_d1, m_d2;
    float m_cornerRadius;
    float m_smoothing;
    float m_cosAngle;
    float m_sinAngle;
    float m_expectedRoundCut;

    float calculateActualSmoothingValue(float allowedCut) const;
    Cubic computeFlankingCurve(float actualRoundCut, float actualSmoothing,
        const Point& corner, const Point& sideStart,
        const Point& circleSegmentIntersection,
        const Point& otherCircleSegmentIntersection, const Point& circleCenter,
        float actualR) const;
    static std::optional<Point> lineIntersection(
        const Point& p0, const Point& d0, const Point& p1, const Point& d1);
};

} // namespace RoundedPolygon
