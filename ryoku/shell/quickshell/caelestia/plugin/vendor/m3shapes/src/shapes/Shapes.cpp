#include "Shapes.hpp"
#include <cmath>
#include <stdexcept>

namespace RoundedPolygon {

RoundedPolygonShape Shapes::circle(
    int numVertices, float radius, float centerX, float centerY) {
    if (numVertices < 3) {
        throw std::invalid_argument("Circle must have at least 3 vertices");
    }

    // Half of the angle between two adjacent vertices on the polygon
    float theta = FloatPi / static_cast<float>(numVertices);
    // Radius of the underlying RoundedPolygon given the desired circle radius
    float polygonRadius = radius / std::cos(theta);

    return RoundedPolygonShape(numVertices, polygonRadius, centerX, centerY,
        CornerRounding(radius, 0.0f));
}

RoundedPolygonShape Shapes::rectangle(float width, float height,
    const CornerRounding& rounding,
    const std::vector<CornerRounding>* perVertexRounding, float centerX,
    float centerY) {
    float halfWidth = width / 2.0f;
    float halfHeight = height / 2.0f;

    std::vector<float> vertices = {
        centerX + halfWidth, centerY + halfHeight, // Top-right
        centerX - halfWidth, centerY + halfHeight, // Top-left
        centerX - halfWidth, centerY - halfHeight, // Bottom-left
        centerX + halfWidth, centerY - halfHeight  // Bottom-right
    };

    return RoundedPolygonShape(
        vertices, rounding, perVertexRounding, centerX, centerY);
}

RoundedPolygonShape Shapes::star(int numVerticesPerRadius, float radius,
    float innerRadius, const CornerRounding& rounding,
    const CornerRounding* innerRounding,
    const std::vector<CornerRounding>* perVertexRounding, float centerX,
    float centerY) {
    if (numVerticesPerRadius < 3) {
        throw std::invalid_argument("Star must have at least 3 points");
    }

    size_t totalVertices = static_cast<size_t>(numVerticesPerRadius) * 2;
    std::vector<float> vertices(totalVertices * 2);
    std::vector<CornerRounding> roundings;

    bool hasPerVertexRounding =
        perVertexRounding && !perVertexRounding->empty();
    bool hasInnerRounding = innerRounding != nullptr;

    if (!hasPerVertexRounding &&
        (rounding != CornerRounding::Unrounded || hasInnerRounding)) {
        roundings.resize(totalVertices);
    }

    for (size_t i = 0; i < totalVertices; ++i) {
        float angleRadians = FloatPi /
                             static_cast<float>(numVerticesPerRadius) *
                             static_cast<float>(i);
        bool isOuter = (i % 2 == 0);
        float r = isOuter ? radius : innerRadius;

        Point vertex =
            radialToCartesian(r, angleRadians, Point(centerX, centerY));
        vertices[i * 2] = vertex.x;
        vertices[i * 2 + 1] = vertex.y;

        if (!hasPerVertexRounding && !roundings.empty()) {
            if (isOuter) {
                roundings[i] = rounding;
            } else {
                roundings[i] = hasInnerRounding ? *innerRounding : rounding;
            }
        }
    }

    if (hasPerVertexRounding) {
        return RoundedPolygonShape(
            vertices, rounding, perVertexRounding, centerX, centerY);
    } else if (!roundings.empty()) {
        return RoundedPolygonShape(
            vertices, CornerRounding::Unrounded, &roundings, centerX, centerY);
    } else {
        return RoundedPolygonShape(
            vertices, rounding, nullptr, centerX, centerY);
    }
}

RoundedPolygonShape Shapes::pill(
    float width, float height, float smoothing, float centerX, float centerY) {
    // A pill is essentially a rectangle with fully rounded ends
    float halfWidth = width / 2.0f;
    float halfHeight = height / 2.0f;
    float radius = std::min(halfWidth, halfHeight);

    // Create vertices for a pill shape
    std::vector<float> vertices = { centerX + halfWidth, centerY + halfHeight,
        centerX - halfWidth, centerY + halfHeight, centerX - halfWidth,
        centerY - halfHeight, centerX + halfWidth, centerY - halfHeight };

    // Full rounding on the ends
    CornerRounding fullRound(radius, smoothing);
    std::vector<CornerRounding> roundings = { fullRound, fullRound, fullRound,
        fullRound };

    return RoundedPolygonShape(
        vertices, CornerRounding::Unrounded, &roundings, centerX, centerY);
}

RoundedPolygonShape Shapes::pillStar(float width, float height,
    int numVerticesPerRadius, float innerRadiusRatio,
    const CornerRounding& rounding, const CornerRounding* innerRounding,
    const std::vector<CornerRounding>* perVertexRounding, float vertexSpacing,
    float startLocation, float centerX, float centerY) {
    if (numVerticesPerRadius < 3) {
        throw std::invalid_argument(
            "PillStar must have at least 3 points per radius");
    }

    // Calculate the effective radii
    float outerWidth = width / 2.0f;
    float outerHeight = height / 2.0f;
    float innerWidth = outerWidth * innerRadiusRatio;
    float innerHeight = outerHeight * innerRadiusRatio;

    size_t totalVertices = static_cast<size_t>(numVerticesPerRadius) * 2;
    std::vector<float> vertices(totalVertices * 2);
    std::vector<CornerRounding> roundings;

    bool hasPerVertexRounding =
        perVertexRounding && !perVertexRounding->empty();
    bool hasInnerRounding = innerRounding != nullptr;

    if (!hasPerVertexRounding &&
        (rounding != CornerRounding::Unrounded || hasInnerRounding)) {
        roundings.resize(totalVertices);
    }

    // Calculate vertices with elliptical distribution
    for (size_t i = 0; i < totalVertices; ++i) {
        float baseAngle =
            TwoPi / static_cast<float>(totalVertices) * static_cast<float>(i) +
            startLocation * TwoPi;
        bool isOuter = (i % 2 == 0);

        float w, h;
        if (isOuter) {
            w = outerWidth;
            h = outerHeight;
        } else {
            // Apply vertex spacing adjustment for inner vertices
            float adjustedIndex =
                static_cast<float>(i) + (vertexSpacing - 0.5f);
            baseAngle =
                TwoPi / static_cast<float>(totalVertices) * adjustedIndex +
                startLocation * TwoPi;
            w = innerWidth;
            h = innerHeight;
        }

        // Elliptical coordinates
        vertices[i * 2] = centerX + w * std::cos(baseAngle);
        vertices[i * 2 + 1] = centerY + h * std::sin(baseAngle);

        if (!hasPerVertexRounding && !roundings.empty()) {
            if (isOuter) {
                roundings[i] = rounding;
            } else {
                roundings[i] = hasInnerRounding ? *innerRounding : rounding;
            }
        }
    }

    if (hasPerVertexRounding) {
        return RoundedPolygonShape(
            vertices, rounding, perVertexRounding, centerX, centerY);
    } else if (!roundings.empty()) {
        return RoundedPolygonShape(
            vertices, CornerRounding::Unrounded, &roundings, centerX, centerY);
    } else {
        return RoundedPolygonShape(
            vertices, rounding, nullptr, centerX, centerY);
    }
}

} // namespace RoundedPolygon
