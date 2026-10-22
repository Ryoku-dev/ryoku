#pragma once

#include "../core/Feature.hpp"
#include "../core/RoundedPolygon.hpp"
#include "FloatMapping.hpp"
#include <memory>
#include <vector>

namespace RoundedPolygon {

// Forward declarations
class MeasuredPolygon;
class Measurer;

/**
 * ProgressableFeature holds a feature along with its progress value
 * along the polygon outline.
 */
struct ProgressableFeature {
    float progress;
    const Feature* feature;

    ProgressableFeature(float p, const Feature* f)
        : progress(p)
        , feature(f) {}
};

/**
 * MeasuredCubic holds information about a cubic curve, including the
 * feature (if any) associated with it, and the outline progress values
 * (start and end) for the cubic.
 */
class MeasuredCubic {
public:
    MeasuredCubic(const Cubic& cubic, float startProgress, float endProgress,
        float measuredSize);

    [[nodiscard]] const Cubic& cubic() const { return m_cubic; }

    [[nodiscard]] float measuredSize() const { return m_measuredSize; }

    [[nodiscard]] float startOutlineProgress() const {
        return m_startOutlineProgress;
    }

    [[nodiscard]] float endOutlineProgress() const {
        return m_endOutlineProgress;
    }

    void updateProgressRange(float startProgress, float endProgress);

    /**
     * Cut this MeasuredCubic at the given outline progress value,
     * returning two new MeasuredCubics.
     */
    [[nodiscard]] std::pair<MeasuredCubic, MeasuredCubic> cutAtProgress(
        float cutOutlineProgress, const Measurer& measurer) const;

private:
    Cubic m_cubic;
    float m_startOutlineProgress;
    float m_endOutlineProgress;
    float m_measuredSize;
};

/**
 * Measurer interface for measuring cubic curves.
 */
class Measurer {
public:
    virtual ~Measurer() = default;

    /**
     * Returns size of given cubic according to the implementation's
     * measurement method (angle, length, etc).
     */
    [[nodiscard]] virtual float measureCubic(const Cubic& c) const = 0;

    /**
     * Given a cubic and a measure that should be between 0 and measureCubic(),
     * finds the parameter t of the cubic at which that measure is reached.
     */
    [[nodiscard]] virtual float findCubicCutPoint(
        const Cubic& c, float m) const = 0;
};

/**
 * LengthMeasurer measures cubics by approximating their arc length.
 */
class LengthMeasurer : public Measurer {
public:
    LengthMeasurer() = default;

    [[nodiscard]] float measureCubic(const Cubic& c) const override;
    [[nodiscard]] float findCubicCutPoint(
        const Cubic& c, float m) const override;

private:
    static constexpr int Segments = 3;

    [[nodiscard]] std::pair<float, float> closestProgressTo(
        const Cubic& cubic, float threshold) const;
};

/**
 * MeasuredPolygon holds a measured representation of a polygon,
 * including cubics with their progress values along the outline.
 */
class MeasuredPolygon {
public:
    [[nodiscard]] const std::vector<MeasuredCubic>& cubics() const {
        return m_cubics;
    }

    [[nodiscard]] const std::vector<ProgressableFeature>& features() const {
        return m_features;
    }

    [[nodiscard]] size_t size() const { return m_cubics.size(); }

    [[nodiscard]] const MeasuredCubic& operator[](size_t index) const {
        return m_cubics[index];
    }

    [[nodiscard]] const MeasuredCubic* getOrNull(size_t index) const {
        return index < m_cubics.size() ? &m_cubics[index] : nullptr;
    }

    /**
     * Cut and shift the polygon at the given cutting point.
     * Returns a new MeasuredPolygon that starts at the cutting point.
     */
    [[nodiscard]] MeasuredPolygon cutAndShift(float cuttingPoint) const;

    /**
     * Create a MeasuredPolygon from a RoundedPolygon using the given measurer.
     */
    [[nodiscard]] static MeasuredPolygon measurePolygon(
        std::shared_ptr<Measurer> measurer, const RoundedPolygonShape& polygon);

private:
    MeasuredPolygon(std::shared_ptr<Measurer> measurer,
        std::vector<ProgressableFeature> features,
        const std::vector<Cubic>& cubics,
        const std::vector<float>& outlineProgress);

    std::shared_ptr<Measurer> m_measurer;
    std::vector<MeasuredCubic> m_cubics;
    std::vector<ProgressableFeature> m_features;
};

} // namespace RoundedPolygon
