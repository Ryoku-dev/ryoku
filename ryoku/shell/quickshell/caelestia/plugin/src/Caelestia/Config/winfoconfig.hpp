#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

// WInfoConfig has no serialized properties (serializer returns {})
// All properties are in AdvancedConfig.winfo
class WInfoConfig : public settings::ObjectNode {
    CONFIG_NODE(WInfoConfig, settings::ObjectNode)
};

} // namespace caelestia::config
