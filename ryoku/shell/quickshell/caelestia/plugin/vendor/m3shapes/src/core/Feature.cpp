#include "Feature.hpp"
#include <cmath>
#include <stdexcept>

namespace RoundedPolygon {

bool Feature::isContinuous(const std::vector<Cubic>& cubics) {
    if (cubics.empty())
        return true;

    const Cubic* prevCubic = &cubics.front();
    for (size_t i = 1; i < cubics.size(); ++i) {
        const Cubic& cubic = cubics[i];
        if (std::abs(cubic.anchor0X() - prevCubic->anchor1X()) >
                DistanceEpsilon ||
            std::abs(cubic.anchor0Y() - prevCubic->anchor1Y()) >
                DistanceEpsilon) {
            return false;
        }
        prevCubic = &cubic;
    }
    return true;
}

std::unique_ptr<Feature> Feature::buildIgnorableFeature(
    const std::vector<Cubic>& cubics) {
    if (cubics.empty()) {
        throw std::invalid_argument("Features need at least one cubic.");
    }
    if (!isContinuous(cubics)) {
        throw std::invalid_argument(
            "Feature must be continuous, with the anchor points of all cubics "
            "matching the anchor points of the preceding and succeeding "
            "cubics");
    }
    return std::make_unique<Edge>(cubics);
}

std::unique_ptr<Feature> Feature::buildEdge(const Cubic& cubic) {
    return std::make_unique<Edge>(std::vector<Cubic>{ cubic });
}

std::unique_ptr<Feature> Feature::buildConvexCorner(
    const std::vector<Cubic>& cubics) {
    if (cubics.empty()) {
        throw std::invalid_argument("Features need at least one cubic.");
    }
    if (!isContinuous(cubics)) {
        throw std::invalid_argument(
            "Feature must be continuous, with the anchor points of all cubics "
            "matching the anchor points of the preceding and succeeding "
            "cubics");
    }
    return std::make_unique<Corner>(cubics, true);
}

std::unique_ptr<Feature> Feature::buildConcaveCorner(
    const std::vector<Cubic>& cubics) {
    if (cubics.empty()) {
        throw std::invalid_argument("Features need at least one cubic.");
    }
    if (!isContinuous(cubics)) {
        throw std::invalid_argument(
            "Feature must be continuous, with the anchor points of all cubics "
            "matching the anchor points of the preceding and succeeding "
            "cubics");
    }
    return std::make_unique<Corner>(cubics, false);
}

// Edge implementation

std::unique_ptr<Feature> Edge::transformed(const PointTransformer& f) const {
    std::vector<Cubic> transformedCubics;
    transformedCubics.reserve(m_cubics.size());
    for (const auto& cubic : m_cubics) {
        transformedCubics.push_back(cubic.transformed(f));
    }
    return std::make_unique<Edge>(transformedCubics);
}

std::unique_ptr<Feature> Edge::reversed() const {
    std::vector<Cubic> reversedCubics;
    reversedCubics.reserve(m_cubics.size());
    for (auto it = m_cubics.rbegin(); it != m_cubics.rend(); ++it) {
        reversedCubics.push_back(it->reverse());
    }
    return std::make_unique<Edge>(reversedCubics);
}

// Corner implementation

std::unique_ptr<Feature> Corner::transformed(const PointTransformer& f) const {
    std::vector<Cubic> transformedCubics;
    transformedCubics.reserve(m_cubics.size());
    for (const auto& cubic : m_cubics) {
        transformedCubics.push_back(cubic.transformed(f));
    }
    return std::make_unique<Corner>(transformedCubics, m_convex);
}

std::unique_ptr<Feature> Corner::reversed() const {
    std::vector<Cubic> reversedCubics;
    reversedCubics.reserve(m_cubics.size());
    for (auto it = m_cubics.rbegin(); it != m_cubics.rend(); ++it) {
        reversedCubics.push_back(it->reverse());
    }
    // Note: convexity is negated when reversing (matching Kotlin
    // implementation)
    return std::make_unique<Corner>(reversedCubics, !m_convex);
}

} // namespace RoundedPolygon
