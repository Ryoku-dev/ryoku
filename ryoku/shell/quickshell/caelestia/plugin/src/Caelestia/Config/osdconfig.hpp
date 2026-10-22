#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class OsdConfig : public settings::ObjectNode {
    CONFIG_NODE(OsdConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, false)
    CONFIG_PROPERTY(int, hideDelay, 2000)
    CONFIG_PROPERTY(int, hoverThickness, 10)
    CONFIG_PROPERTY(int, hoverWidth, 50)
    CONFIG_PROPERTY(bool, enableBrightness, true)
    CONFIG_PROPERTY(bool, enableMicrophone, false)
    CONFIG_PROPERTY(bool, enableVolume, true)
};

} // namespace caelestia::config
