#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class TabSwitchConfig : public settings::ObjectNode {
    CONFIG_NODE(TabSwitchConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(bool, currentDesktopOnly, false)
    CONFIG_PROPERTY(bool, previewOnDesktop, true)
    CONFIG_PROPERTY(bool, showMinimized, true)
    CONFIG_PROPERTY(bool, allScreens, true)
    CONFIG_PROPERTY(QString, layout, QStringLiteral("caelestia"))
};

} // namespace caelestia::config
