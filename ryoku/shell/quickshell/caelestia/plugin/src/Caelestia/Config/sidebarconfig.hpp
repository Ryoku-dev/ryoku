#pragma once

#include <qstring.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;

class SidebarConfig : public settings::ObjectNode {
    CONFIG_NODE(SidebarConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(int, dragThreshold, 50)
    CONFIG_PROPERTY(int, grabWidth, 12)
    CONFIG_PROPERTY(QString, defaultTab, u"last"_s)
    CONFIG_PROPERTY(bool, pinned, false)
    CONFIG_PROPERTY(int, width, 0)
};

} // namespace caelestia::config
