#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class OverviewConfig : public settings::ObjectNode {
    CONFIG_NODE(OverviewConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(bool, disableWallpaperBlur, true)
    CONFIG_PROPERTY(bool, enableOverviewBlur, true)
    CONFIG_PROPERTY(bool, showOnHover, true)
    CONFIG_PROPERTY(int, dragThreshold, 30)
    CONFIG_PROPERTY(int, hoverThickness, 20)
    CONFIG_PROPERTY(bool, hoverTopLeft, true)
    CONFIG_PROPERTY(bool, hoverTopRight, false)
    CONFIG_PROPERTY(bool, hoverBottomLeft, false)
    CONFIG_PROPERTY(bool, hoverBottomRight, false)

    CONFIG_PROPERTY(int, baseDuration, 300)
    CONFIG_PROPERTY(qreal, blobScaleSpeed, 1.0)
    CONFIG_PROPERTY(qreal, wallpaperFadeSpeed, 1.0)
    CONFIG_PROPERTY(qreal, gridFadeSpeed, 1.0)
    CONFIG_PROPERTY(int, easingType, 2)
    CONFIG_PROPERTY(int, layoutType, 1)
};

} // namespace caelestia::config
