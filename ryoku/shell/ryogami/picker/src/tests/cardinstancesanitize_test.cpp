#include "../render/cardinstancesanitize.h"

#include <limits>
#include <vector>

int main()
{
    CardInstance valid;
    valid.rect[2] = 100.0f;
    valid.rect[3] = 60.0f;
    valid.misc[0] = CardTex::Near;
    valid.misc[1] = 1;

    CardInstance nonFinite = valid;
    nonFinite.quadA[2] = std::numeric_limits<float>::quiet_NaN();

    CardInstance huge = valid;
    huge.rect[0] = 2000000.0f;

    CardInstance staleLayer = valid;
    staleLayer.misc[1] = 9;

    std::vector<CardInstance> instances{valid, nonFinite, huge, staleLayer};
    const std::size_t dropped = sanitizeCardInstances(instances, 2, 1, false);
    if (dropped != 2 || instances.size() != 2)
        return 1;
    if (instances[0].misc[0] != CardTex::Near)
        return 2;
    if (instances[1].misc[0] != CardTex::None || instances[1].misc[1] != 0)
        return 3;
    return 0;
}
