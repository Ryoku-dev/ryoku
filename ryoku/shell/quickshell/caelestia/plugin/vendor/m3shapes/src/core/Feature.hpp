#pragma once

#define FEATURE_H

#include "Cubic.hpp"
#include <memory>
#include <vector>

namespace RoundedPolygon {

/**
 * Feature represents a group of cubic curves that form part of a polygon's
 * outline. Features can be edges (straight sections) or corners (rounded
 * sections). This grouping is used by the Morph class to map similar features
 * between shapes.
 */
class Feature {
public:
    virtual ~Feature() = default;

    [[nodiscard]] const std::vector<Cubic>& cubics() const { return m_cubics; }

    // Transform this feature with a point transformer
    [[nodiscard]] virtual std::unique_ptr<Feature> transformed(
        const PointTransformer& f) const = 0;

    // Reverse the direction of this feature
    [[nodiscard]] virtual std::unique_ptr<Feature> reversed() const = 0;

    // Feature type queries
    [[nodiscard]] virtual bool isIgnorableFeature() const = 0;
    [[nodiscard]] virtual bool isEdge() const = 0;
    [[nodiscard]] virtual bool isConvexCorner() const = 0;
    [[nodiscard]] virtual bool isConcaveCorner() const = 0;

    // Factory methods for creating features
    [[nodiscard]] static std::unique_ptr<Feature> buildIgnorableFeature(
        const std::vector<Cubic>& cubics);
    [[nodiscard]] static std::unique_ptr<Feature> buildEdge(const Cubic& cubic);
    [[nodiscard]] static std::unique_ptr<Feature> buildConvexCorner(
        const std::vector<Cubic>& cubics);
    [[nodiscard]] static std::unique_ptr<Feature> buildConcaveCorner(
        const std::vector<Cubic>& cubics);

protected:
    explicit Feature(const std::vector<Cubic>& cubics)
        : m_cubics(cubics) {}

    std::vector<Cubic> m_cubics;

private:
    static bool isContinuous(const std::vector<Cubic>& cubics);
};

/**
 * Edge represents a straight section between corners.
 * Edges are considered ignorable in morph mapping.
 */
class Edge : public Feature {
public:
    explicit Edge(const std::vector<Cubic>& cubics)
        : Feature(cubics) {}

    [[nodiscard]] std::unique_ptr<Feature> transformed(
        const PointTransformer& f) const override;
    [[nodiscard]] std::unique_ptr<Feature> reversed() const override;

    [[nodiscard]] bool isIgnorableFeature() const override { return true; }

    [[nodiscard]] bool isEdge() const override { return true; }

    [[nodiscard]] bool isConvexCorner() const override { return false; }

    [[nodiscard]] bool isConcaveCorner() const override { return false; }
};

/**
 * Corner represents a rounded corner section.
 * Corners can be either convex (outward) or concave (inward).
 */
class Corner : public Feature {
public:
    Corner(const std::vector<Cubic>& cubics, bool convex)
        : Feature(cubics)
        , m_convex(convex) {}

    [[nodiscard]] bool isConvex() const { return m_convex; }

    [[nodiscard]] std::unique_ptr<Feature> transformed(
        const PointTransformer& f) const override;
    [[nodiscard]] std::unique_ptr<Feature> reversed() const override;

    [[nodiscard]] bool isIgnorableFeature() const override { return false; }

    [[nodiscard]] bool isEdge() const override { return false; }

    [[nodiscard]] bool isConvexCorner() const override { return m_convex; }

    [[nodiscard]] bool isConcaveCorner() const override { return !m_convex; }

private:
    bool m_convex;
};

} // namespace RoundedPolygon
