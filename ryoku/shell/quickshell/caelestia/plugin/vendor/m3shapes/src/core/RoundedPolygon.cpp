#include "RoundedPolygon.hpp"
#include <algorithm>
#include <cmath>
#include <limits>
#include <optional>
#include <stdexcept>

namespace RoundedPolygon {

// RoundedPolygonShape implementation

RoundedPolygonShape::RoundedPolygonShape(
    std::vector<std::unique_ptr<Feature>> features, const Point& center)
    : m_features(std::move(features))
    , m_center(center) {
    buildCubics();
}

RoundedPolygonShape::RoundedPolygonShape(int numVertices, float radius,
    float centerX, float centerY, const CornerRounding& rounding,
    const std::vector<CornerRounding>* perVertexRounding)
    : RoundedPolygonShape(
          verticesFromNumVerts(numVertices, radius, centerX, centerY), rounding,
          perVertexRounding, centerX, centerY) {}

RoundedPolygonShape::RoundedPolygonShape(const std::vector<float>& vertices,
    const CornerRounding& rounding,
    const std::vector<CornerRounding>* perVertexRounding, float centerX,
    float centerY) {
    if (vertices.size() < 6) {
        throw std::invalid_argument("Polygons must have at least 3 vertices");
    }
    if (vertices.size() % 2 == 1) {
        throw std::invalid_argument("The vertices array should have even size");
    }
    if (perVertexRounding && perVertexRounding->size() * 2 != vertices.size()) {
        throw std::invalid_argument(
            "perVertexRounding list should be either null or "
            "the same size as the number of vertices (vertices.size / 2)");
    }

    const size_t n = vertices.size() / 2;
    std::vector<std::vector<Cubic>> corners;
    std::vector<RoundedCorner> roundedCorners;

    // Create rounded corners
    for (size_t i = 0; i < n; ++i) {
        const CornerRounding& vtxRounding =
            perVertexRounding ? (*perVertexRounding)[i] : rounding;
        size_t prevIndex = ((i + n - 1) % n) * 2;
        size_t nextIndex = ((i + 1) % n) * 2;

        roundedCorners.emplace_back(
            Point(vertices[prevIndex], vertices[prevIndex + 1]),
            Point(vertices[i * 2], vertices[i * 2 + 1]),
            Point(vertices[nextIndex], vertices[nextIndex + 1]), vtxRounding);
    }

    // Calculate cut adjustments
    std::vector<std::pair<float, float>> cutAdjusts;
    for (size_t ix = 0; ix < n; ++ix) {
        float expectedRoundCut =
            roundedCorners[ix].expectedRoundCut() +
            roundedCorners[(ix + 1) % n].expectedRoundCut();
        float expectedCut = roundedCorners[ix].expectedCut() +
                            roundedCorners[(ix + 1) % n].expectedCut();

        float vtxX = vertices[ix * 2];
        float vtxY = vertices[ix * 2 + 1];
        float nextVtxX = vertices[((ix + 1) % n) * 2];
        float nextVtxY = vertices[((ix + 1) % n) * 2 + 1];
        float sideSize = distance(vtxX - nextVtxX, vtxY - nextVtxY);

        if (expectedRoundCut > sideSize) {
            cutAdjusts.emplace_back(sideSize / expectedRoundCut, 0.0f);
        } else if (expectedCut > sideSize) {
            cutAdjusts.emplace_back(1.0f, (sideSize - expectedRoundCut) /
                                              (expectedCut - expectedRoundCut));
        } else {
            cutAdjusts.emplace_back(1.0f, 1.0f);
        }
    }

    // Create beziers for each rounded corner
    for (size_t i = 0; i < n; ++i) {
        std::vector<float> allowedCuts(2);
        for (size_t delta = 0; delta <= 1; ++delta) {
            auto [roundCutRatio, cutRatio] =
                cutAdjusts[(i + n - 1 + delta) % n];
            allowedCuts[delta] =
                roundedCorners[i].expectedRoundCut() * roundCutRatio +
                (roundedCorners[i].expectedCut() -
                    roundedCorners[i].expectedRoundCut()) *
                    cutRatio;
        }
        corners.push_back(
            roundedCorners[i].getCubics(allowedCuts[0], allowedCuts[1]));
    }

    // Build features
    for (size_t i = 0; i < n; ++i) {
        size_t prevVtxIndex = (i + n - 1) % n;
        size_t nextVtxIndex = (i + 1) % n;
        Point currVertex(vertices[i * 2], vertices[i * 2 + 1]);
        Point prevVertex(
            vertices[prevVtxIndex * 2], vertices[prevVtxIndex * 2 + 1]);
        Point nextVertex(
            vertices[nextVtxIndex * 2], vertices[nextVtxIndex * 2 + 1]);
        bool isConvex = convex(prevVertex, currVertex, nextVertex);

        if (isConvex) {
            m_features.push_back(Feature::buildConvexCorner(corners[i]));
        } else {
            m_features.push_back(Feature::buildConcaveCorner(corners[i]));
        }

        // Add edge
        std::vector<Cubic> edgeCubics = { Cubic::straightLine(
            corners[i].back().anchor1X(), corners[i].back().anchor1Y(),
            corners[(i + 1) % n].front().anchor0X(),
            corners[(i + 1) % n].front().anchor0Y()) };
        m_features.push_back(Feature::buildEdge(edgeCubics[0]));
    }

    // Set center
    constexpr float lowest = std::numeric_limits<float>::lowest();
    constexpr float epsilon = 1e-30f;
    if (std::abs(centerX - lowest) < epsilon ||
        std::abs(centerY - lowest) < epsilon) {
        m_center = calculateCenterFromVertices(vertices);
    } else {
        m_center = Point(centerX, centerY);
    }

    buildCubics();
}

RoundedPolygonShape::RoundedPolygonShape(const RoundedPolygonShape& other)
    : m_center(other.m_center)
    , m_cubics(other.m_cubics) {
    for (const auto& feature : other.m_features) {
        // Clone features via transform identity
        m_features.push_back(feature->transformed([](float x, float y) {
            return TransformResult(x, y);
        }));
    }
}

RoundedPolygonShape::RoundedPolygonShape(RoundedPolygonShape&& other) noexcept
    : m_features(std::move(other.m_features))
    , m_center(other.m_center)
    , m_cubics(std::move(other.m_cubics)) {}

RoundedPolygonShape& RoundedPolygonShape::operator=(
    const RoundedPolygonShape& other) {
    if (this != &other) {
        m_center = other.m_center;
        m_cubics = other.m_cubics;
        m_features.clear();
        for (const auto& feature : other.m_features) {
            m_features.push_back(feature->transformed([](float x, float y) {
                return TransformResult(x, y);
            }));
        }
    }
    return *this;
}

RoundedPolygonShape& RoundedPolygonShape::operator=(
    RoundedPolygonShape&& other) noexcept {
    if (this != &other) {
        m_features = std::move(other.m_features);
        m_center = other.m_center;
        m_cubics = std::move(other.m_cubics);
    }
    return *this;
}

void RoundedPolygonShape::buildCubics() {
    m_cubics.clear();

    // Track first and last non-zero cubics (stored by value, not pointer)
    std::optional<Cubic> firstCubic;
    std::optional<Cubic> lastCubic;

    std::vector<Cubic> firstFeatureSplitStart;
    std::vector<Cubic> firstFeatureSplitEnd;

    if (!m_features.empty() && m_features[0]->cubics().size() == 3) {
        const Cubic& centerCubic = m_features[0]->cubics()[1];
        auto [start, end] = centerCubic.split(0.5f);
        firstFeatureSplitStart = { m_features[0]->cubics()[0], start };
        firstFeatureSplitEnd = { end, m_features[0]->cubics()[2] };
    }

    for (size_t i = 0; i <= m_features.size(); ++i) {
        const std::vector<Cubic>* featureCubics = nullptr;

        if (i == 0 && !firstFeatureSplitEnd.empty()) {
            featureCubics = &firstFeatureSplitEnd;
        } else if (i == m_features.size()) {
            if (!firstFeatureSplitStart.empty()) {
                featureCubics = &firstFeatureSplitStart;
            } else {
                break;
            }
        } else {
            featureCubics = &m_features[i]->cubics();
        }

        for (const auto& cubic : *featureCubics) {
            if (!cubic.zeroLength()) {
                if (lastCubic) {
                    m_cubics.push_back(*lastCubic);
                }
                lastCubic = cubic;
                if (!firstCubic) {
                    firstCubic = cubic;
                }
            } else if (lastCubic) {
                // Update lastCubic's endpoint to match this zero-length cubic
                lastCubic->points()[6] = cubic.anchor1X();
                lastCubic->points()[7] = cubic.anchor1Y();
            }
        }
    }

    if (lastCubic && firstCubic) {
        // Add final cubic that closes the shape by connecting back to first
        m_cubics.push_back(Cubic(lastCubic->anchor0X(), lastCubic->anchor0Y(),
            lastCubic->control0X(), lastCubic->control0Y(),
            lastCubic->control1X(), lastCubic->control1Y(),
            firstCubic->anchor0X(), firstCubic->anchor0Y()));
    } else {
        // Empty / 0-sized polygon
        m_cubics.push_back(Cubic::empty(m_center.x, m_center.y));
    }
}

RoundedPolygonShape RoundedPolygonShape::transformed(
    const PointTransformer& f) const {
    auto transformedCenter = ::RoundedPolygon::transformed(m_center, f);
    std::vector<std::unique_ptr<Feature>> transformedFeatures;
    for (const auto& feature : m_features) {
        transformedFeatures.push_back(feature->transformed(f));
    }
    return RoundedPolygonShape(
        std::move(transformedFeatures), transformedCenter);
}

RoundedPolygonShape RoundedPolygonShape::normalized() const {
    auto bounds = calculateBounds();
    float width = bounds[2] - bounds[0];
    float height = bounds[3] - bounds[1];
    float side = std::max(width, height);
    float offsetX = (side - width) / 2.0f - bounds[0];
    float offsetY = (side - height) / 2.0f - bounds[1];

    return transformed([side, offsetX, offsetY](float x, float y) {
        return TransformResult((x + offsetX) / side, (y + offsetY) / side);
    });
}

void RoundedPolygonShape::calculateBounds(
    std::array<float, 4>& bounds, bool approximate) const {
    float minX = std::numeric_limits<float>::max();
    float minY = std::numeric_limits<float>::max();
    float maxX = std::numeric_limits<float>::lowest();
    float maxY = std::numeric_limits<float>::lowest();

    std::array<float, 4> cubicBounds;
    for (const auto& cubic : m_cubics) {
        cubic.calculateBounds(cubicBounds, approximate);
        minX = std::min(minX, cubicBounds[0]);
        minY = std::min(minY, cubicBounds[1]);
        maxX = std::max(maxX, cubicBounds[2]);
        maxY = std::max(maxY, cubicBounds[3]);
    }

    bounds[0] = minX;
    bounds[1] = minY;
    bounds[2] = maxX;
    bounds[3] = maxY;
}

std::array<float, 4> RoundedPolygonShape::calculateBounds(
    bool approximate) const {
    std::array<float, 4> bounds;
    calculateBounds(bounds, approximate);
    return bounds;
}

void RoundedPolygonShape::calculateMaxBounds(
    std::array<float, 4>& bounds) const {
    float maxDistSquared = 0.0f;
    for (const auto& cubic : m_cubics) {
        float anchorDist = distanceSquared(
            cubic.anchor0X() - m_center.x, cubic.anchor0Y() - m_center.y);
        Point middlePoint = cubic.pointOnCurve(0.5f);
        float middleDist = distanceSquared(
            middlePoint.x - m_center.x, middlePoint.y - m_center.y);
        maxDistSquared =
            std::max(maxDistSquared, std::max(anchorDist, middleDist));
    }

    float dist = std::sqrt(maxDistSquared);
    bounds[0] = m_center.x - dist;
    bounds[1] = m_center.y - dist;
    bounds[2] = m_center.x + dist;
    bounds[3] = m_center.y + dist;
}

std::array<float, 4> RoundedPolygonShape::calculateMaxBounds() const {
    std::array<float, 4> bounds;
    calculateMaxBounds(bounds);
    return bounds;
}

Point RoundedPolygonShape::calculateCenterFromVertices(
    const std::vector<float>& vertices) {
    float cumulativeX = 0.0f;
    float cumulativeY = 0.0f;
    for (size_t i = 0; i < vertices.size(); i += 2) {
        cumulativeX += vertices[i];
        cumulativeY += vertices[i + 1];
    }
    float numPoints = static_cast<float>(vertices.size()) / 2.0f;
    return Point(cumulativeX / numPoints, cumulativeY / numPoints);
}

std::vector<float> RoundedPolygonShape::verticesFromNumVerts(
    int numVertices, float radius, float centerX, float centerY) {
    std::vector<float> result(static_cast<size_t>(numVertices) * 2);
    for (size_t i = 0; i < static_cast<size_t>(numVertices); ++i) {
        Point vertex = radialToCartesian(
            radius, FloatPi / static_cast<float>(numVertices) * 2.0f *
                        static_cast<float>(i));
        result[i * 2] = vertex.x + centerX;
        result[i * 2 + 1] = vertex.y + centerY;
    }
    return result;
}

// RoundedCorner implementation

RoundedCorner::RoundedCorner(const Point& p0, const Point& p1, const Point& p2,
    const CornerRounding& rounding)
    : m_p0(p0)
    , m_p1(p1)
    , m_p2(p2) {
    Point v01 = p0 - p1;
    Point v21 = p2 - p1;
    float d01 = v01.getDistance();
    float d21 = v21.getDistance();

    if (d01 > 0.0f && d21 > 0.0f) {
        m_d1 = v01 / d01;
        m_d2 = v21 / d21;
        m_cornerRadius = rounding.radius;
        m_smoothing = rounding.smoothing;

        m_cosAngle = m_d1.dotProduct(m_d2);
        m_sinAngle = std::sqrt(1.0f - square(m_cosAngle));

        if (m_sinAngle > 1e-3f) {
            m_expectedRoundCut =
                m_cornerRadius * (m_cosAngle + 1.0f) / m_sinAngle;
        } else {
            m_expectedRoundCut = 0.0f;
        }
    } else {
        m_d1 = Point(0, 0);
        m_d2 = Point(0, 0);
        m_cornerRadius = 0.0f;
        m_smoothing = 0.0f;
        m_cosAngle = 0.0f;
        m_sinAngle = 0.0f;
        m_expectedRoundCut = 0.0f;
    }
}

std::vector<Cubic> RoundedCorner::getCubics(
    float allowedCut0, float allowedCut1) const {
    float allowedCut = std::min(allowedCut0, allowedCut1);

    if (m_expectedRoundCut < DistanceEpsilon || allowedCut < DistanceEpsilon ||
        m_cornerRadius < DistanceEpsilon) {
        return { Cubic::straightLine(m_p1.x, m_p1.y, m_p1.x, m_p1.y) };
    }

    float actualRoundCut = std::min(allowedCut, m_expectedRoundCut);
    float actualSmoothing0 = calculateActualSmoothingValue(allowedCut0);
    float actualSmoothing1 = calculateActualSmoothingValue(allowedCut1);
    float actualR = m_cornerRadius * actualRoundCut / m_expectedRoundCut;

    float centerDistance = std::sqrt(square(actualR) + square(actualRoundCut));
    Point center =
        m_p1 + ((m_d1 + m_d2) / 2.0f).getDirection() * centerDistance;

    Point circleIntersection0 = m_p1 + m_d1 * actualRoundCut;
    Point circleIntersection2 = m_p1 + m_d2 * actualRoundCut;

    Cubic flanking0 = computeFlankingCurve(actualRoundCut, actualSmoothing0,
        m_p1, m_p0, circleIntersection0, circleIntersection2, center, actualR);

    Cubic flanking2 = computeFlankingCurve(actualRoundCut, actualSmoothing1,
        m_p1, m_p2, circleIntersection2, circleIntersection0, center, actualR)
                          .reverse();

    return { flanking0,
        Cubic::circularArc(center.x, center.y, flanking0.anchor1X(),
            flanking0.anchor1Y(), flanking2.anchor0X(), flanking2.anchor0Y()),
        flanking2 };
}

float RoundedCorner::calculateActualSmoothingValue(float allowedCut) const {
    float expCut = expectedCut();
    if (allowedCut > expCut) {
        return m_smoothing;
    } else if (allowedCut > m_expectedRoundCut) {
        return m_smoothing * (allowedCut - m_expectedRoundCut) /
               (expCut - m_expectedRoundCut);
    }
    return 0.0f;
}

Cubic RoundedCorner::computeFlankingCurve(float actualRoundCut,
    float actualSmoothing, const Point& corner, const Point& sideStart,
    const Point& circleSegmentIntersection,
    const Point& otherCircleSegmentIntersection, const Point& circleCenter,
    float actualR) const {
    Point sideDirection = (sideStart - corner).getDirection();
    Point curveStart =
        corner + sideDirection * actualRoundCut * (1.0f + actualSmoothing);

    Point p = interpolate(circleSegmentIntersection,
        (circleSegmentIntersection + otherCircleSegmentIntersection) / 2.0f,
        actualSmoothing);

    Point curveEnd =
        circleCenter +
        directionVector(p.x - circleCenter.x, p.y - circleCenter.y) * actualR;

    Point circleTangent = (curveEnd - circleCenter).rotate90();
    auto intersection =
        lineIntersection(sideStart, sideDirection, curveEnd, circleTangent);
    Point anchorEnd = intersection.value_or(circleSegmentIntersection);

    Point anchorStart = (curveStart + anchorEnd * 2.0f) / 3.0f;

    return Cubic(curveStart, anchorStart, anchorEnd, curveEnd);
}

std::optional<Point> RoundedCorner::lineIntersection(
    const Point& p0, const Point& d0, const Point& p1, const Point& d1) {
    Point rotatedD1 = d1.rotate90();
    float den = d0.dotProduct(rotatedD1);
    if (std::abs(den) < DistanceEpsilon) {
        return std::nullopt;
    }

    float num = (p1 - p0).dotProduct(rotatedD1);
    if (std::abs(den) < DistanceEpsilon * std::abs(num)) {
        return std::nullopt;
    }

    float k = num / den;
    return p0 + d0 * k;
}

} // namespace RoundedPolygon
