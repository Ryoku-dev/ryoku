#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class NexusConfig : public settings::ObjectNode {
    CONFIG_NODE(NexusConfig, settings::ObjectNode)

    CONFIG_PROPERTY(int, wallpapersPerRow, 4)
    CONFIG_GLOBAL_PROPERTY(int, networkRescanInterval, 15000)
    CONFIG_PROPERTY(int, maxNetworksShown, 5)
};

} // namespace caelestia::config
