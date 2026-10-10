#include "cardinstancesanitize.h"

#include <algorithm>
#include <cmath>

namespace {

bool finiteArray(const float *values, std::size_t count, float limit)
{
    for (std::size_t i = 0; i < count; ++i)
        if (!std::isfinite(values[i]) || std::abs(values[i]) > limit)
            return false;
    return true;
}

bool usableGeometry(const CardInstance &inst, float limit)
{
    const float *groups[] = {
        inst.rect, inst.radii, inst.fill, inst.tint, inst.border, inst.params, inst.uv,
        inst.crop, inst.flip, inst.shape, inst.quadA, inst.quadB, inst.quadW, inst.quadL,
    };
    for (const float *group : groups)
        if (!finiteArray(group, 4, limit))
            return false;

    if (inst.rect[2] < 0.0f || inst.rect[3] < 0.0f)
        return false;
    if ((inst.misc[3] & CardFlag::Projected) != 0u) {
        for (float w : inst.quadW)
            if (w < 0.0001f)
                return false;
    }
    return true;
}

bool normalizedRect(const float *rect)
{
    constexpr float tolerance = 0.0001f;
    return rect[0] >= -tolerance && rect[1] >= -tolerance
        && rect[2] >= 0.0f && rect[3] >= 0.0f
        && rect[0] + rect[2] <= 1.0f + tolerance
        && rect[1] + rect[3] <= 1.0f + tolerance;
}

} // namespace

std::size_t sanitizeCardInstances(std::vector<CardInstance> &instances, int nearLayers,
                                  int farLayers, bool previewAvailable, float coordinateLimit)
{
    const auto before = instances.size();
    instances.erase(std::remove_if(instances.begin(), instances.end(), [=](CardInstance &inst) {
        if (!usableGeometry(inst, coordinateLimit))
            return true;

        const uint32_t source = inst.misc[0];
        const bool missing = source > CardTex::Preview
            || (source == CardTex::Near && inst.misc[1] >= uint32_t(std::max(nearLayers, 0)))
            || (source == CardTex::Far && inst.misc[1] >= uint32_t(std::max(farLayers, 0)))
            || (source == CardTex::Preview && !previewAvailable)
            || (source != CardTex::None && (!normalizedRect(inst.uv) || !normalizedRect(inst.crop)));
        if (missing) {
            inst.misc[0] = CardTex::None;
            inst.misc[1] = 0;
        }
        return false;
    }), instances.end());
    return before - instances.size();
}
