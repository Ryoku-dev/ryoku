#include "Morph.hpp"
#include <algorithm>
#include <memory>
#include <optional>
#include <stdexcept>

namespace RoundedPolygon {

Morph::Morph(const RoundedPolygonShape& start, const RoundedPolygonShape& end)
    : m_start(start)
    , m_end(end)
    , m_morphMatch(match(start, end)) {}

std::vector<Cubic> Morph::asCubics(float progress) const {
    std::vector<Cubic> result;
    result.reserve(m_morphMatch.size());

    Cubic* firstCubic = nullptr;
    Cubic* lastCubic = nullptr;

    for (size_t i = 0; i < m_morphMatch.size(); ++i) {
        const auto& [startCubic, endCubic] = m_morphMatch[i];

        // Interpolate all 8 points
        std::array<float, 8> points;
        for (size_t j = 0; j < 8; ++j) {
            points[j] = interpolate(
                startCubic.points()[j], endCubic.points()[j], progress);
        }

        result.emplace_back(points);

        if (firstCubic == nullptr) {
            firstCubic = &result.back();
        }
        lastCubic = &result.back();
    }

    // Ensure the last point matches the first point exactly
    // to avoid rendering artifacts
    if (lastCubic != nullptr && firstCubic != nullptr) {
        result.back() = Cubic(lastCubic->anchor0X(), lastCubic->anchor0Y(),
            lastCubic->control0X(), lastCubic->control0Y(),
            lastCubic->control1X(), lastCubic->control1Y(),
            firstCubic->anchor0X(), firstCubic->anchor0Y());
    }

    return result;
}

void Morph::forEachCubic(float progress,
    const std::function<void(const MutableCubic&)>& callback) const {
    MutableCubic mutableCubic;

    for (const auto& [startCubic, endCubic] : m_morphMatch) {
        mutableCubic.interpolate(startCubic, endCubic, progress);
        callback(mutableCubic);
    }
}

std::array<float, 4> Morph::calculateBounds(bool approximate) const {
    auto startBounds = m_start.calculateBounds(approximate);
    auto endBounds = m_end.calculateBounds(approximate);

    return { std::min(startBounds[0], endBounds[0]),
        std::min(startBounds[1], endBounds[1]),
        std::max(startBounds[2], endBounds[2]),
        std::max(startBounds[3], endBounds[3]) };
}

std::array<float, 4> Morph::calculateMaxBounds() const {
    auto startBounds = m_start.calculateMaxBounds();
    auto endBounds = m_end.calculateMaxBounds();

    return { std::min(startBounds[0], endBounds[0]),
        std::min(startBounds[1], endBounds[1]),
        std::max(startBounds[2], endBounds[2]),
        std::max(startBounds[3], endBounds[3]) };
}

std::vector<std::pair<Cubic, Cubic>> Morph::match(
    const RoundedPolygonShape& p1, const RoundedPolygonShape& p2) {
    // Measure polygons to get progress values for each cubic
    auto measurer = std::make_shared<LengthMeasurer>();
    auto measuredPolygon1 = MeasuredPolygon::measurePolygon(measurer, p1);
    auto measuredPolygon2 = MeasuredPolygon::measurePolygon(measurer, p2);

    // Get features for mapping
    const auto& features1 = measuredPolygon1.features();
    const auto& features2 = measuredPolygon2.features();

    // Map features between shapes
    DoubleMapper doubleMapper = featureMapper(features1, features2);

    // Find the cut point on polygon 2 that corresponds to progress 0 on
    // polygon 1
    float polygon2CutPoint = doubleMapper.map(0.0f);

    // Cut and rotate polygon 2 so it aligns with polygon 1
    MeasuredPolygon bs1 = measuredPolygon1;
    MeasuredPolygon bs2 = measuredPolygon2.cutAndShift(polygon2CutPoint);

    // Match cubics between the two shapes
    std::vector<std::pair<Cubic, Cubic>> result;

    size_t i1 = 0;
    size_t i2 = 0;

    // Use optional to track current cubic state (including cut results)
    std::optional<MeasuredCubic> b1Opt;
    std::optional<MeasuredCubic> b2Opt;

    if (i1 < bs1.size()) {
        b1Opt = bs1[i1++];
    }
    if (i2 < bs2.size()) {
        b2Opt = bs2[i2++];
    }

    while (b1Opt.has_value() && b2Opt.has_value()) {
        const MeasuredCubic& b1 = *b1Opt;
        const MeasuredCubic& b2 = *b2Opt;

        // Get end progress values (in shape1's perspective)
        float b1a = (i1 == bs1.size()) ? 1.0f : b1.endOutlineProgress();
        float b2a;
        if (i2 == bs2.size()) {
            b2a = 1.0f;
        } else {
            b2a = doubleMapper.mapBack(positiveModulo(
                b2.endOutlineProgress() + polygon2CutPoint, 1.0f));
        }

        float minb = std::min(b1a, b2a);

        // Cut and get segments
        MeasuredCubic seg1 = b1;
        MeasuredCubic seg2 = b2;

        if (b1a > minb + AngleEpsilon) {
            auto [cut1, cut2] = b1.cutAtProgress(minb, *measurer);
            seg1 = cut1;
            b1Opt = cut2;
        } else {
            if (i1 < bs1.size()) {
                b1Opt = bs1[i1++];
            } else {
                b1Opt.reset();
            }
        }

        if (b2a > minb + AngleEpsilon) {
            auto [cut1, cut2] = b2.cutAtProgress(
                positiveModulo(doubleMapper.map(minb) - polygon2CutPoint, 1.0f),
                *measurer);
            seg2 = cut1;
            b2Opt = cut2;
        } else {
            if (i2 < bs2.size()) {
                b2Opt = bs2[i2++];
            } else {
                b2Opt.reset();
            }
        }

        result.emplace_back(seg1.cubic(), seg2.cubic());
    }

    if (b1Opt.has_value() || b2Opt.has_value()) {
        throw std::runtime_error(
            "Expected both polygon's cubics to be fully matched");
    }

    return result;
}

} // namespace RoundedPolygon
