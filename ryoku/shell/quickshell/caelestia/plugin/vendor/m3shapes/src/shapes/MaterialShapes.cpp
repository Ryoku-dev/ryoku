#include "MaterialShapes.hpp"
#include <cmath>

namespace RoundedPolygon {

// Pre-defined corner roundings
static const CornerRounding cornerRound15(0.15f);
static const CornerRounding cornerRound20(0.2f);
static const CornerRounding cornerRound30(0.3f);
static const CornerRounding cornerRound50(0.5f);
static const CornerRounding cornerRound100(1.0f);

RoundedPolygonShape MaterialShapes::rotated(
    const RoundedPolygonShape& shape, float degrees) {
    float radians = degrees * FloatPi / 180.0f;
    float cosA = std::cos(radians);
    float sinA = std::sin(radians);
    return shape.transformed([cosA, sinA](float x, float y) {
        return TransformResult(x * cosA - y * sinA, x * sinA + y * cosA);
    });
}

std::vector<MaterialShapes::PointNRound> MaterialShapes::doRepeat(
    const std::vector<PointNRound>& points, int reps, float centerX,
    float centerY, bool mirroring) {
    std::vector<PointNRound> result;

    if (mirroring) {
        // Calculate angles and distances
        std::vector<float> angles;
        std::vector<float> distances;
        for (const auto& p : points) {
            float dx = p.x - centerX;
            float dy = p.y - centerY;
            angles.push_back(std::atan2(dy, dx) * 180.0f / FloatPi);
            distances.push_back(std::sqrt(dx * dx + dy * dy));
        }

        size_t actualReps = static_cast<size_t>(reps) * 2;
        float sectionAngle = 360.0f / static_cast<float>(actualReps);

        for (size_t rep = 0; rep < actualReps; ++rep) {
            for (size_t index = 0; index < points.size(); ++index) {
                size_t i = (rep % 2 == 0) ? index : points.size() - 1 - index;
                if (i > 0 || rep % 2 == 0) {
                    float a;
                    if (rep % 2 == 0) {
                        a = sectionAngle * static_cast<float>(rep) + angles[i];
                    } else {
                        a = sectionAngle * static_cast<float>(rep) +
                            sectionAngle - angles[i] + 2 * angles[0];
                    }
                    float rad = a * FloatPi / 180.0f;
                    float finalX = std::cos(rad) * distances[i] + centerX;
                    float finalY = std::sin(rad) * distances[i] + centerY;
                    result.emplace_back(finalX, finalY, points[i].rounding);
                }
            }
        }
    } else {
        size_t np = points.size();
        for (size_t rep = 0; rep < static_cast<size_t>(reps); ++rep) {
            for (size_t i = 0; i < np; ++i) {
                float angle =
                    360.0f / static_cast<float>(reps) * static_cast<float>(rep);
                float rad = angle * FloatPi / 180.0f;
                float dx = points[i].x - centerX;
                float dy = points[i].y - centerY;
                float newX = dx * std::cos(rad) - dy * std::sin(rad) + centerX;
                float newY = dx * std::sin(rad) + dy * std::cos(rad) + centerY;
                result.emplace_back(newX, newY, points[i].rounding);
            }
        }
    }

    return result;
}

RoundedPolygonShape MaterialShapes::customPolygon(
    const std::vector<PointNRound>& pnr, int reps, float centerX, float centerY,
    bool mirroring) {
    auto actualPoints = doRepeat(pnr, reps, centerX, centerY, mirroring);

    std::vector<float> vertices;
    std::vector<CornerRounding> roundings;
    vertices.reserve(actualPoints.size() * 2);
    roundings.reserve(actualPoints.size());

    for (const auto& p : actualPoints) {
        vertices.push_back(p.x);
        vertices.push_back(p.y);
        roundings.push_back(p.rounding);
    }

    return RoundedPolygonShape(
        vertices, CornerRounding::Unrounded, &roundings, centerX, centerY);
}

RoundedPolygonShape MaterialShapes::circle() {
    return Shapes::circle(10).normalized();
}

RoundedPolygonShape MaterialShapes::square() {
    return Shapes::rectangle(1.0f, 1.0f, cornerRound30).normalized();
}

RoundedPolygonShape MaterialShapes::slanted() {
    return customPolygon(
        { PointNRound(0.926f, 0.970f, CornerRounding(0.189f, 0.811f)),
            PointNRound(-0.021f, 0.967f, CornerRounding(0.187f, 0.057f)) },
        2)
        .normalized();
}

RoundedPolygonShape MaterialShapes::arch() {
    std::vector<CornerRounding> pvr = { cornerRound100, cornerRound100,
        cornerRound20, cornerRound20 };
    return rotated(RoundedPolygonShape(
                       4, 1.0f, 0.0f, 0.0f, CornerRounding::Unrounded, &pvr),
        -135.0f)
        .normalized();
}

RoundedPolygonShape MaterialShapes::fan() {
    return customPolygon(
        { PointNRound(1.004f, 1.000f, CornerRounding(0.148f, 0.417f)),
            PointNRound(0.000f, 1.000f, CornerRounding(0.151f)),
            PointNRound(0.000f, -0.003f, CornerRounding(0.148f)),
            PointNRound(0.978f, 0.020f, CornerRounding(0.803f)) },
        1)
        .normalized();
}

RoundedPolygonShape MaterialShapes::arrow() {
    return customPolygon(
        { PointNRound(0.500f, 0.892f, CornerRounding(0.313f)),
            PointNRound(-0.216f, 1.050f, CornerRounding(0.207f)),
            PointNRound(0.499f, -0.160f, CornerRounding(0.215f, 1.000f)),
            PointNRound(1.225f, 1.060f, CornerRounding(0.211f)) },
        1)
        .normalized();
}

RoundedPolygonShape MaterialShapes::semiCircle() {
    std::vector<CornerRounding> pvr = { cornerRound20, cornerRound20,
        cornerRound100, cornerRound100 };
    return Shapes::rectangle(1.6f, 1.0f, CornerRounding::Unrounded, &pvr)
        .normalized();
}

RoundedPolygonShape MaterialShapes::oval() {
    auto shape = Shapes::circle(8);
    // Scale Y axis
    auto scaled = shape.transformed([](float x, float y) {
        return TransformResult(x, y * 0.64f);
    });
    return rotated(scaled, -45.0f).normalized();
}

RoundedPolygonShape MaterialShapes::pill() {
    return customPolygon(
        { PointNRound(0.961f, 0.039f, CornerRounding(0.426f)),
            PointNRound(1.001f, 0.428f),
            PointNRound(1.000f, 0.609f, CornerRounding(1.000f)) },
        2, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::triangle() {
    return rotated(
        RoundedPolygonShape(3, 1.0f, 0.0f, 0.0f, cornerRound20), -90.0f)
        .normalized();
}

RoundedPolygonShape MaterialShapes::diamond() {
    return customPolygon(
        { PointNRound(0.500f, 1.096f, CornerRounding(0.151f, 0.524f)),
            PointNRound(0.040f, 0.500f, CornerRounding(0.159f)) },
        2)
        .normalized();
}

RoundedPolygonShape MaterialShapes::clamShell() {
    return customPolygon(
        { PointNRound(0.171f, 0.841f, CornerRounding(0.159f)),
            PointNRound(-0.020f, 0.500f, CornerRounding(0.140f)),
            PointNRound(0.170f, 0.159f, CornerRounding(0.159f)) },
        2)
        .normalized();
}

RoundedPolygonShape MaterialShapes::pentagon() {
    return customPolygon(
        { PointNRound(0.500f, -0.009f, CornerRounding(0.172f)),
            PointNRound(1.030f, 0.365f, CornerRounding(0.164f)),
            PointNRound(0.828f, 0.970f, CornerRounding(0.169f)) },
        1, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::gem() {
    return customPolygon(
        { PointNRound(0.499f, 1.023f, CornerRounding(0.241f, 0.778f)),
            PointNRound(-0.005f, 0.792f, CornerRounding(0.208f)),
            PointNRound(0.073f, 0.258f, CornerRounding(0.228f)),
            PointNRound(0.433f, -0.000f, CornerRounding(0.491f)) },
        1, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::sunny() {
    return Shapes::star(8, 1.0f, 0.8f, cornerRound15).normalized();
}

RoundedPolygonShape MaterialShapes::verySunny() {
    return customPolygon(
        { PointNRound(0.500f, 1.080f, CornerRounding(0.085f)),
            PointNRound(0.358f, 0.843f, CornerRounding(0.085f)) },
        8)
        .normalized();
}

RoundedPolygonShape MaterialShapes::cookie4Sided() {
    return customPolygon(
        { PointNRound(1.237f, 1.236f, CornerRounding(0.258f)),
            PointNRound(0.500f, 0.918f, CornerRounding(0.233f)) },
        4)
        .normalized();
}

RoundedPolygonShape MaterialShapes::cookie6Sided() {
    return customPolygon(
        { PointNRound(0.723f, 0.884f, CornerRounding(0.394f)),
            PointNRound(0.500f, 1.099f, CornerRounding(0.398f)) },
        6)
        .normalized();
}

RoundedPolygonShape MaterialShapes::cookie7Sided() {
    return rotated(Shapes::star(7, 1.0f, 0.75f, cornerRound50), -90.0f)
        .normalized();
}

RoundedPolygonShape MaterialShapes::cookie9Sided() {
    return rotated(Shapes::star(9, 1.0f, 0.8f, cornerRound50), -90.0f)
        .normalized();
}

RoundedPolygonShape MaterialShapes::cookie12Sided() {
    return rotated(Shapes::star(12, 1.0f, 0.8f, cornerRound50), -90.0f)
        .normalized();
}

RoundedPolygonShape MaterialShapes::ghostish() {
    return customPolygon(
        { PointNRound(0.500f, 0.0f, CornerRounding(1.000f)),
            PointNRound(1.0f, 0.0f, CornerRounding(1.000f)),
            PointNRound(1.0f, 1.140f, CornerRounding(0.254f, 0.106f)),
            PointNRound(0.575f, 0.906f, CornerRounding(0.253f)) },
        1, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::clover4Leaf() {
    return customPolygon(
        { PointNRound(0.500f, 0.074f),
            PointNRound(0.725f, -0.099f, CornerRounding(0.476f)) },
        4, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::clover8Leaf() {
    return customPolygon(
        { PointNRound(0.500f, 0.036f),
            PointNRound(0.758f, -0.101f, CornerRounding(0.209f)) },
        8)
        .normalized();
}

RoundedPolygonShape MaterialShapes::burst() {
    return customPolygon(
        { PointNRound(0.500f, -0.006f, CornerRounding(0.006f)),
            PointNRound(0.592f, 0.158f, CornerRounding(0.006f)) },
        12)
        .normalized();
}

RoundedPolygonShape MaterialShapes::softBurst() {
    return customPolygon(
        { PointNRound(0.193f, 0.277f, CornerRounding(0.053f)),
            PointNRound(0.176f, 0.055f, CornerRounding(0.053f)) },
        10)
        .normalized();
}

RoundedPolygonShape MaterialShapes::boom() {
    return customPolygon(
        { PointNRound(0.457f, 0.296f, CornerRounding(0.007f)),
            PointNRound(0.500f, -0.051f, CornerRounding(0.007f)) },
        15)
        .normalized();
}

RoundedPolygonShape MaterialShapes::softBoom() {
    return customPolygon(
        { PointNRound(0.733f, 0.454f),
            PointNRound(0.839f, 0.437f, CornerRounding(0.532f)),
            PointNRound(0.949f, 0.449f, CornerRounding(0.439f, 1.000f)),
            PointNRound(0.998f, 0.478f, CornerRounding(0.174f)) },
        16, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::flower() {
    return customPolygon(
        { PointNRound(0.370f, 0.187f),
            PointNRound(0.416f, 0.049f, CornerRounding(0.381f)),
            PointNRound(0.479f, 0.001f, CornerRounding(0.095f)) },
        8, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::puffy() {
    auto shape =
        customPolygon({ PointNRound(0.500f, 0.053f),
                          PointNRound(0.545f, -0.040f, CornerRounding(0.405f)),
                          PointNRound(0.670f, -0.035f, CornerRounding(0.426f)),
                          PointNRound(0.717f, 0.066f, CornerRounding(0.574f)),
                          PointNRound(0.722f, 0.128f),
                          PointNRound(0.777f, 0.002f, CornerRounding(0.360f)),
                          PointNRound(0.914f, 0.149f, CornerRounding(0.660f)),
                          PointNRound(0.926f, 0.289f, CornerRounding(0.660f)),
                          PointNRound(0.881f, 0.346f),
                          PointNRound(0.940f, 0.344f, CornerRounding(0.126f)),
                          PointNRound(1.003f, 0.437f, CornerRounding(0.255f)) },
            2, 0.5f, 0.5f, true);
    // Scale Y
    return shape
        .transformed([](float x, float y) {
            return TransformResult(x, y * 0.742f);
        })
        .normalized();
}

RoundedPolygonShape MaterialShapes::puffyDiamond() {
    return customPolygon(
        { PointNRound(0.870f, 0.130f, CornerRounding(0.146f)),
            PointNRound(0.818f, 0.357f),
            PointNRound(1.000f, 0.332f, CornerRounding(0.853f)) },
        4, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::pixelCircle() {
    return customPolygon(
        { PointNRound(0.500f, 0.000f), PointNRound(0.704f, 0.000f),
            PointNRound(0.704f, 0.065f), PointNRound(0.843f, 0.065f),
            PointNRound(0.843f, 0.148f), PointNRound(0.926f, 0.148f),
            PointNRound(0.926f, 0.296f), PointNRound(1.000f, 0.296f) },
        2, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::pixelTriangle() {
    return customPolygon(
        { PointNRound(0.110f, 0.500f), PointNRound(0.113f, 0.000f),
            PointNRound(0.287f, 0.000f), PointNRound(0.287f, 0.087f),
            PointNRound(0.421f, 0.087f), PointNRound(0.421f, 0.170f),
            PointNRound(0.560f, 0.170f), PointNRound(0.560f, 0.265f),
            PointNRound(0.674f, 0.265f), PointNRound(0.675f, 0.344f),
            PointNRound(0.789f, 0.344f), PointNRound(0.789f, 0.439f),
            PointNRound(0.888f, 0.439f) },
        1, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::bun() {
    return customPolygon(
        { PointNRound(0.796f, 0.500f),
            PointNRound(0.853f, 0.518f, CornerRounding(1.0f)),
            PointNRound(0.992f, 0.631f, CornerRounding(1.0f)),
            PointNRound(0.968f, 1.000f, CornerRounding(1.0f)) },
        2, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::heart() {
    return customPolygon(
        { PointNRound(0.500f, 0.268f, CornerRounding(0.016f)),
            PointNRound(0.792f, -0.066f, CornerRounding(0.958f)),
            PointNRound(1.064f, 0.276f, CornerRounding(1.000f)),
            PointNRound(0.501f, 0.946f, CornerRounding(0.129f)) },
        1, 0.5f, 0.5f, true)
        .normalized();
}

RoundedPolygonShape MaterialShapes::getShape(ShapeType type) {
    switch (type) {
    case ShapeType::Circle:
        return circle();
    case ShapeType::Square:
        return square();
    case ShapeType::Slanted:
        return slanted();
    case ShapeType::Arch:
        return arch();
    case ShapeType::Fan:
        return fan();
    case ShapeType::Arrow:
        return arrow();
    case ShapeType::SemiCircle:
        return semiCircle();
    case ShapeType::Oval:
        return oval();
    case ShapeType::Pill:
        return pill();
    case ShapeType::Triangle:
        return triangle();
    case ShapeType::Diamond:
        return diamond();
    case ShapeType::ClamShell:
        return clamShell();
    case ShapeType::Pentagon:
        return pentagon();
    case ShapeType::Gem:
        return gem();
    case ShapeType::Sunny:
        return sunny();
    case ShapeType::VerySunny:
        return verySunny();
    case ShapeType::Cookie4Sided:
        return cookie4Sided();
    case ShapeType::Cookie6Sided:
        return cookie6Sided();
    case ShapeType::Cookie7Sided:
        return cookie7Sided();
    case ShapeType::Cookie9Sided:
        return cookie9Sided();
    case ShapeType::Cookie12Sided:
        return cookie12Sided();
    case ShapeType::Ghostish:
        return ghostish();
    case ShapeType::Clover4Leaf:
        return clover4Leaf();
    case ShapeType::Clover8Leaf:
        return clover8Leaf();
    case ShapeType::Burst:
        return burst();
    case ShapeType::SoftBurst:
        return softBurst();
    case ShapeType::Boom:
        return boom();
    case ShapeType::SoftBoom:
        return softBoom();
    case ShapeType::Flower:
        return flower();
    case ShapeType::Puffy:
        return puffy();
    case ShapeType::PuffyDiamond:
        return puffyDiamond();
    case ShapeType::PixelCircle:
        return pixelCircle();
    case ShapeType::PixelTriangle:
        return pixelTriangle();
    case ShapeType::Bun:
        return bun();
    case ShapeType::Heart:
        return heart();
    default:
        return circle();
    }
}

} // namespace RoundedPolygon
