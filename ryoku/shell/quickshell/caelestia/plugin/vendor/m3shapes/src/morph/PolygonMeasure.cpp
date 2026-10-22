#include "PolygonMeasure.hpp"
#include <algorithm>
#include <cmath>
#include <stdexcept>

namespace RoundedPolygon {

// MeasuredCubic implementation

MeasuredCubic::MeasuredCubic(const Cubic& cubic, float startProgress,
    float endProgress, float measuredSize)
    : m_cubic(cubic)
    , m_startOutlineProgress(startProgress)
    , m_endOutlineProgress(endProgress)
    , m_measuredSize(measuredSize) {
    if (endProgress < startProgress) {
        throw std::invalid_argument(
            "endOutlineProgress must be >= startOutlineProgress");
    }
}

void MeasuredCubic::updateProgressRange(
    float startProgress, float endProgress) {
    if (endProgress < startProgress) {
        throw std::invalid_argument(
            "endOutlineProgress must be >= startOutlineProgress");
    }
    m_startOutlineProgress = startProgress;
    m_endOutlineProgress = endProgress;
}

std::pair<MeasuredCubic, MeasuredCubic> MeasuredCubic::cutAtProgress(
    float cutOutlineProgress, const Measurer& measurer) const {
    // Bound the cut progress to this cubic's range
    float boundedCutProgress = std::clamp(
        cutOutlineProgress, m_startOutlineProgress, m_endOutlineProgress);

    float outlineProgressSize = m_endOutlineProgress - m_startOutlineProgress;
    float progressFromStart = boundedCutProgress - m_startOutlineProgress;

    // Calculate relative progress within this cubic
    float relativeProgress = progressFromStart / outlineProgressSize;
    float t =
        measurer.findCubicCutPoint(m_cubic, relativeProgress * m_measuredSize);

    if (t < 0.0f || t > 1.0f) {
        throw std::runtime_error("Cubic cut point must be between 0 and 1");
    }

    // Split the cubic
    auto [c1, c2] = m_cubic.split(t);

    return { MeasuredCubic(c1, m_startOutlineProgress, boundedCutProgress,
                 measurer.measureCubic(c1)),
        MeasuredCubic(c2, boundedCutProgress, m_endOutlineProgress,
            measurer.measureCubic(c2)) };
}

// LengthMeasurer implementation

float LengthMeasurer::measureCubic(const Cubic& c) const {
    return closestProgressTo(c, std::numeric_limits<float>::infinity()).second;
}

float LengthMeasurer::findCubicCutPoint(const Cubic& c, float m) const {
    return closestProgressTo(c, m).first;
}

std::pair<float, float> LengthMeasurer::closestProgressTo(
    const Cubic& cubic, float threshold) const {
    float total = 0.0f;
    float remainder = threshold;
    Point prev(cubic.anchor0X(), cubic.anchor0Y());

    for (size_t i = 1; i <= static_cast<size_t>(Segments); ++i) {
        float progress = static_cast<float>(i) / static_cast<float>(Segments);
        Point point = cubic.pointOnCurve(progress);
        float segment = (point - prev).getDistance();

        if (segment >= remainder) {
            return { progress - (1.0f - remainder / segment) /
                                    static_cast<float>(Segments),
                threshold };
        }

        remainder -= segment;
        total += segment;
        prev = point;
    }

    return { 1.0f, total };
}

// MeasuredPolygon implementation

MeasuredPolygon::MeasuredPolygon(std::shared_ptr<Measurer> measurer,
    std::vector<ProgressableFeature> features, const std::vector<Cubic>& cubics,
    const std::vector<float>& outlineProgress)
    : m_measurer(std::move(measurer))
    , m_features(std::move(features)) {

    if (outlineProgress.size() != cubics.size() + 1) {
        throw std::invalid_argument(
            "Outline progress size must be cubics size + 1");
    }
    if (outlineProgress.front() != 0.0f) {
        throw std::invalid_argument(
            "First outline progress value must be zero");
    }
    if (outlineProgress.back() != 1.0f) {
        throw std::invalid_argument("Last outline progress value must be one");
    }

    float startOutlineProgress = 0.0f;
    for (size_t i = 0; i < cubics.size(); ++i) {
        // Filter out "empty" cubics
        if ((outlineProgress[i + 1] - outlineProgress[i]) > DistanceEpsilon) {
            m_cubics.emplace_back(cubics[i], startOutlineProgress,
                outlineProgress[i + 1], m_measurer->measureCubic(cubics[i]));
            startOutlineProgress = outlineProgress[i + 1];
        }
    }

    // Ensure the last cubic ends at 1.0
    if (!m_cubics.empty()) {
        m_cubics.back().updateProgressRange(
            m_cubics.back().startOutlineProgress(), 1.0f);
    } else {
        throw std::runtime_error("No cubics in measured polygon");
    }
}

MeasuredPolygon MeasuredPolygon::cutAndShift(float cuttingPoint) const {
    if (cuttingPoint < 0.0f || cuttingPoint > 1.0f) {
        throw std::invalid_argument("Cutting point must be between 0 and 1");
    }
    if (cuttingPoint < DistanceEpsilon) {
        return *this;
    }

    // Find the cubic to cut
    size_t targetIndex = 0;
    for (size_t i = 0; i < m_cubics.size(); ++i) {
        if (cuttingPoint >= m_cubics[i].startOutlineProgress() &&
            cuttingPoint <= m_cubics[i].endOutlineProgress()) {
            targetIndex = i;
            break;
        }
    }

    const auto& target = m_cubics[targetIndex];

    // Cut the target cubic
    auto [b1, b2] = target.cutAtProgress(cuttingPoint, *m_measurer);

    // Build new cubics list
    std::vector<Cubic> retCubics;
    retCubics.push_back(b2.cubic());

    for (size_t i = 1; i < m_cubics.size(); ++i) {
        retCubics.push_back(
            m_cubics[(i + targetIndex) % m_cubics.size()].cubic());
    }
    retCubics.push_back(b1.cubic());

    // Build new outline progress
    std::vector<float> retOutlineProgress;
    retOutlineProgress.reserve(m_cubics.size() + 2);

    for (size_t index = 0; index < m_cubics.size() + 2; ++index) {
        if (index == 0) {
            retOutlineProgress.push_back(0.0f);
        } else if (index == m_cubics.size() + 1) {
            retOutlineProgress.push_back(1.0f);
        } else {
            size_t cubicIndex = (targetIndex + index - 1) % m_cubics.size();
            retOutlineProgress.push_back(positiveModulo(
                m_cubics[cubicIndex].endOutlineProgress() - cuttingPoint,
                1.0f));
        }
    }

    // Shift features
    std::vector<ProgressableFeature> newFeatures;
    for (const auto& feature : m_features) {
        newFeatures.emplace_back(
            positiveModulo(feature.progress - cuttingPoint, 1.0f),
            feature.feature);
    }

    return MeasuredPolygon(
        m_measurer, std::move(newFeatures), retCubics, retOutlineProgress);
}

MeasuredPolygon MeasuredPolygon::measurePolygon(
    std::shared_ptr<Measurer> measurer, const RoundedPolygonShape& polygon) {
    std::vector<Cubic> cubics;
    std::vector<std::pair<const Feature*, size_t>> featureToCubic;

    // Get cubics from the polygon and extract features
    for (const auto& feature : polygon.features()) {
        const auto& featureCubics = feature->cubics();
        for (size_t cubicIndex = 0; cubicIndex < featureCubics.size();
            ++cubicIndex) {
            // For corners, use the middle cubic as representative
            auto* corner = dynamic_cast<const Corner*>(feature.get());
            if (corner != nullptr && cubicIndex == featureCubics.size() / 2) {
                featureToCubic.emplace_back(feature.get(), cubics.size());
            }
            cubics.push_back(featureCubics[cubicIndex]);
        }
    }

    // Measure all cubics
    std::vector<float> measures;
    measures.push_back(0.0f);
    for (const auto& cubic : cubics) {
        float measure = measurer->measureCubic(cubic);
        if (measure < 0.0f) {
            throw std::runtime_error("Measured cubic must be >= 0");
        }
        measures.push_back(measures.back() + measure);
    }

    float totalMeasure = measures.back();

    // Convert to outline progress
    std::vector<float> outlineProgress;
    outlineProgress.reserve(measures.size());
    for (float measure : measures) {
        outlineProgress.push_back(measure / totalMeasure);
    }

    // Build features with progress
    std::vector<ProgressableFeature> features;
    for (const auto& [feature, idx] : featureToCubic) {
        float progress = positiveModulo(
            (outlineProgress[idx] + outlineProgress[idx + 1]) / 2.0f, 1.0f);
        features.emplace_back(progress, feature);
    }

    return MeasuredPolygon(
        measurer, std::move(features), cubics, outlineProgress);
}

} // namespace RoundedPolygon
