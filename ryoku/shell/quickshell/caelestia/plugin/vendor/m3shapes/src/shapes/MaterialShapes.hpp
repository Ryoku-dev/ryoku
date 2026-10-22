#pragma once

#include "Shapes.hpp"

namespace RoundedPolygon {

/**
 * MaterialShapes provides 35 predefined Material Design shape presets.
 * All shapes are normalized to fit within a unit square (0,0)-(1,1).
 */
class MaterialShapes {
public:
    // Singleton accessors for each shape (lazily initialized)
    [[nodiscard]] static RoundedPolygonShape circle();
    [[nodiscard]] static RoundedPolygonShape square();
    [[nodiscard]] static RoundedPolygonShape slanted();
    [[nodiscard]] static RoundedPolygonShape arch();
    [[nodiscard]] static RoundedPolygonShape fan();
    [[nodiscard]] static RoundedPolygonShape arrow();
    [[nodiscard]] static RoundedPolygonShape semiCircle();
    [[nodiscard]] static RoundedPolygonShape oval();
    [[nodiscard]] static RoundedPolygonShape pill();
    [[nodiscard]] static RoundedPolygonShape triangle();
    [[nodiscard]] static RoundedPolygonShape diamond();
    [[nodiscard]] static RoundedPolygonShape clamShell();
    [[nodiscard]] static RoundedPolygonShape pentagon();
    [[nodiscard]] static RoundedPolygonShape gem();
    [[nodiscard]] static RoundedPolygonShape sunny();
    [[nodiscard]] static RoundedPolygonShape verySunny();
    [[nodiscard]] static RoundedPolygonShape cookie4Sided();
    [[nodiscard]] static RoundedPolygonShape cookie6Sided();
    [[nodiscard]] static RoundedPolygonShape cookie7Sided();
    [[nodiscard]] static RoundedPolygonShape cookie9Sided();
    [[nodiscard]] static RoundedPolygonShape cookie12Sided();
    [[nodiscard]] static RoundedPolygonShape ghostish();
    [[nodiscard]] static RoundedPolygonShape clover4Leaf();
    [[nodiscard]] static RoundedPolygonShape clover8Leaf();
    [[nodiscard]] static RoundedPolygonShape burst();
    [[nodiscard]] static RoundedPolygonShape softBurst();
    [[nodiscard]] static RoundedPolygonShape boom();
    [[nodiscard]] static RoundedPolygonShape softBoom();
    [[nodiscard]] static RoundedPolygonShape flower();
    [[nodiscard]] static RoundedPolygonShape puffy();
    [[nodiscard]] static RoundedPolygonShape puffyDiamond();
    [[nodiscard]] static RoundedPolygonShape pixelCircle();
    [[nodiscard]] static RoundedPolygonShape pixelTriangle();
    [[nodiscard]] static RoundedPolygonShape bun();
    [[nodiscard]] static RoundedPolygonShape heart();

    // Shape name enumeration for QML
    enum class ShapeType {
        Circle,
        Square,
        Slanted,
        Arch,
        Fan,
        Arrow,
        SemiCircle,
        Oval,
        Pill,
        Triangle,
        Diamond,
        ClamShell,
        Pentagon,
        Gem,
        Sunny,
        VerySunny,
        Cookie4Sided,
        Cookie6Sided,
        Cookie7Sided,
        Cookie9Sided,
        Cookie12Sided,
        Ghostish,
        Clover4Leaf,
        Clover8Leaf,
        Burst,
        SoftBurst,
        Boom,
        SoftBoom,
        Flower,
        Puffy,
        PuffyDiamond,
        PixelCircle,
        PixelTriangle,
        Bun,
        Heart
    };

    // Get shape by type
    [[nodiscard]] static RoundedPolygonShape getShape(ShapeType type);

    // Helper struct for custom polygon construction
    struct PointNRound {
        float x, y;
        CornerRounding rounding;

        PointNRound(float x, float y,
            const CornerRounding& r = CornerRounding::Unrounded)
            : x(x)
            , y(y)
            , rounding(r) {}
    };

    /**
     * Create a custom polygon from points with optional rotational repetition.
     * @param points Vector of points with optional per-vertex rounding
     * @param reps Number of rotational repetitions (1 = no repetition)
     * @param centerX Center X for rotation (default 0.5)
     * @param centerY Center Y for rotation (default 0.5)
     * @param mirroring If true, alternate repetitions are mirrored
     */
    [[nodiscard]] static RoundedPolygonShape customPolygon(
        const std::vector<PointNRound>& points, int reps, float centerX = 0.5f,
        float centerY = 0.5f, bool mirroring = false);

private:
    MaterialShapes() = default;

    // Helper function to rotate points
    static std::vector<PointNRound> doRepeat(
        const std::vector<PointNRound>& points, int reps, float centerX,
        float centerY, bool mirroring);

    // Rotation helper
    static RoundedPolygonShape rotated(
        const RoundedPolygonShape& shape, float degrees);
};

} // namespace RoundedPolygon
