#include "FloatMapping.hpp"
#include <algorithm>
#include <cmath>
#include <stdexcept>

namespace RoundedPolygon {

bool progressInRange(float progress, float progressFrom, float progressTo) {
    if (progressTo >= progressFrom) {
        return progress >= progressFrom && progress <= progressTo;
    } else {
        return progress >= progressFrom || progress <= progressTo;
    }
}

float progressDistance(float p1, float p2) {
    float diff = std::abs(p1 - p2);
    return std::min(diff, 1.0f - diff);
}

void validateProgress(const std::vector<float>& p) {
    if (p.size() < 2) {
        return;
    }

    float prev = p.back();
    int wraps = 0;

    for (size_t i = 0; i < p.size(); ++i) {
        float curr = p[i];

        if (curr < 0.0f || curr >= 1.0f) {
            throw std::invalid_argument(
                "FloatMapping - Progress outside of range [0, 1)");
        }

        if (progressDistance(curr, prev) <= DistanceEpsilon) {
            throw std::invalid_argument(
                "FloatMapping - Progress repeats a value");
        }

        if (curr < prev) {
            wraps++;
            if (wraps > 1) {
                throw std::invalid_argument(
                    "FloatMapping - Progress wraps more than once");
            }
        }
        prev = curr;
    }
}

float linearMap(const std::vector<float>& xValues,
    const std::vector<float>& yValues, float x) {
    if (x < 0.0f || x > 1.0f) {
        throw std::invalid_argument("Invalid progress value");
    }

    // Find the segment that contains x
    size_t segmentStartIndex = 0;
    for (size_t i = 0; i < xValues.size(); ++i) {
        size_t nextIdx = (i + 1) % xValues.size();
        if (progressInRange(x, xValues[i], xValues[nextIdx])) {
            segmentStartIndex = i;
            break;
        }
    }

    size_t segmentEndIndex = (segmentStartIndex + 1) % xValues.size();

    float segmentSizeX = positiveModulo(
        xValues[segmentEndIndex] - xValues[segmentStartIndex], 1.0f);
    float segmentSizeY = positiveModulo(
        yValues[segmentEndIndex] - yValues[segmentStartIndex], 1.0f);

    float positionInSegment;
    if (segmentSizeX < 0.001f) {
        positionInSegment = 0.5f;
    } else {
        positionInSegment =
            positiveModulo(x - xValues[segmentStartIndex], 1.0f) / segmentSizeX;
    }

    return positiveModulo(
        yValues[segmentStartIndex] + segmentSizeY * positionInSegment, 1.0f);
}

DoubleMapper::DoubleMapper(
    const std::vector<std::pair<float, float>>& mappings) {
    m_sourceValues.reserve(mappings.size());
    m_targetValues.reserve(mappings.size());

    for (const auto& mapping : mappings) {
        m_sourceValues.push_back(mapping.first);
        m_targetValues.push_back(mapping.second);
    }

    validateProgress(m_sourceValues);
    validateProgress(m_targetValues);
}

float DoubleMapper::map(float x) const {
    return linearMap(m_sourceValues, m_targetValues, x);
}

float DoubleMapper::mapBack(float x) const {
    return linearMap(m_targetValues, m_sourceValues, x);
}

const DoubleMapper DoubleMapper::Identity({ { 0.0f, 0.0f }, { 0.5f, 0.5f } });

} // namespace RoundedPolygon
