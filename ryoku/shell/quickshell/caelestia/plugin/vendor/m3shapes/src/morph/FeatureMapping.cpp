#include "FeatureMapping.hpp"
#include <algorithm>
#include <limits>
#include <set>

namespace RoundedPolygon {

namespace {

struct DistanceVertex {
    float distance;
    const ProgressableFeature* f1;
    const ProgressableFeature* f2;

    bool operator<(const DistanceVertex& other) const {
        return distance < other.distance;
    }
};

class MappingHelper {
public:
    std::vector<std::pair<float, float>> mapping;

    void addMapping(
        const ProgressableFeature* f1, const ProgressableFeature* f2) {
        // Don't map the same feature twice
        if (m_usedF1.count(f1) > 0 || m_usedF2.count(f2) > 0) {
            return;
        }

        // Find insertion point (keep sorted by first element)
        auto it = std::lower_bound(mapping.begin(), mapping.end(), f1->progress,
            [](const std::pair<float, float>& pair, float val) {
                return pair.first < val;
            });

        size_t insertionIndex =
            static_cast<size_t>(std::distance(mapping.begin(), it));
        size_t n = mapping.size();

        // Can always add first element
        if (n >= 1) {
            size_t beforeIdx = (insertionIndex + n - 1) % n;
            size_t afterIdx = insertionIndex % n;
            float before1 = mapping[beforeIdx].first;
            float before2 = mapping[beforeIdx].second;
            float after1 = mapping[afterIdx].first;
            float after2 = mapping[afterIdx].second;

            // Don't add features too close to each other
            if (progressDistance(f1->progress, before1) < DistanceEpsilon ||
                progressDistance(f1->progress, after1) < DistanceEpsilon ||
                progressDistance(f2->progress, before2) < DistanceEpsilon ||
                progressDistance(f2->progress, after2) < DistanceEpsilon) {
                return;
            }

            // Check for crossings when we have 2+ elements
            if (n > 1 && !progressInRange(f2->progress, before2, after2)) {
                return;
            }
        }

        // Add the mapping
        mapping.insert(it, { f1->progress, f2->progress });
        m_usedF1.insert(f1);
        m_usedF2.insert(f2);
    }

private:
    std::set<const ProgressableFeature*> m_usedF1;
    std::set<const ProgressableFeature*> m_usedF2;
};

std::vector<std::pair<float, float>> doMapping(
    const std::vector<const ProgressableFeature*>& features1,
    const std::vector<const ProgressableFeature*>& features2) {

    // Build distance list for all feature pairs
    std::vector<DistanceVertex> distanceVertexList;
    for (const auto* f1 : features1) {
        for (const auto* f2 : features2) {
            float d = featureDistSquared(f1->feature, f2->feature);
            if (d < std::numeric_limits<float>::max()) {
                distanceVertexList.push_back({ d, f1, f2 });
            }
        }
    }

    // Sort by distance
    std::sort(distanceVertexList.begin(), distanceVertexList.end());

    // Special cases
    if (distanceVertexList.empty()) {
        return { { 0.0f, 0.0f }, { 0.5f, 0.5f } };
    }

    if (distanceVertexList.size() == 1) {
        float f1 = distanceVertexList[0].f1->progress;
        float f2 = distanceVertexList[0].f2->progress;
        return { { f1, f2 },
            { std::fmod(f1 + 0.5f, 1.0f), std::fmod(f2 + 0.5f, 1.0f) } };
    }

    // Build mapping using greedy algorithm
    MappingHelper helper;
    for (const auto& vertex : distanceVertexList) {
        helper.addMapping(vertex.f1, vertex.f2);
    }

    return helper.mapping;
}

} // anonymous namespace

DoubleMapper featureMapper(const std::vector<ProgressableFeature>& features1,
    const std::vector<ProgressableFeature>& features2) {
    // Filter to only corners
    std::vector<const ProgressableFeature*> filteredFeatures1;
    for (const auto& f : features1) {
        if (dynamic_cast<const Corner*>(f.feature) != nullptr) {
            filteredFeatures1.push_back(&f);
        }
    }

    std::vector<const ProgressableFeature*> filteredFeatures2;
    for (const auto& f : features2) {
        if (dynamic_cast<const Corner*>(f.feature) != nullptr) {
            filteredFeatures2.push_back(&f);
        }
    }

    auto featureProgressMapping =
        doMapping(filteredFeatures1, filteredFeatures2);

    return DoubleMapper(featureProgressMapping);
}

float featureDistSquared(const Feature* f1, const Feature* f2) {
    auto* corner1 = dynamic_cast<const Corner*>(f1);
    auto* corner2 = dynamic_cast<const Corner*>(f2);

    // Don't match convex to concave corners
    if (corner1 != nullptr && corner2 != nullptr &&
        corner1->isConvex() != corner2->isConvex()) {
        return std::numeric_limits<float>::max();
    }

    Point p1 = featureRepresentativePoint(f1);
    Point p2 = featureRepresentativePoint(f2);

    return (p1 - p2).getDistanceSquared();
}

Point featureRepresentativePoint(const Feature* feature) {
    const auto& cubics = feature->cubics();
    if (cubics.empty()) {
        return Point(0.0f, 0.0f);
    }
    float x = (cubics.front().anchor0X() + cubics.back().anchor1X()) / 2.0f;
    float y = (cubics.front().anchor0Y() + cubics.back().anchor1Y()) / 2.0f;
    return Point(x, y);
}

} // namespace RoundedPolygon
