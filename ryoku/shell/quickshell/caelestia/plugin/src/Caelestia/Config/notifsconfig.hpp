#pragma once

#include <qstring.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class NotifsConfig : public settings::ObjectNode {
    CONFIG_NODE(NotifsConfig, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(bool, expire, true)
    CONFIG_GLOBAL_PROPERTY(QString, fullscreen, QStringLiteral("off"))
    CONFIG_GLOBAL_PROPERTY(QString, monitor, QStringLiteral("all"))
    CONFIG_GLOBAL_PROPERTY(int, defaultExpireTimeout, 5000)
    CONFIG_GLOBAL_PROPERTY(int, fullscreenExpireTimeout, 2000)
    CONFIG_PROPERTY(qreal, clearThreshold, 0.3)
    CONFIG_PROPERTY(int, expandThreshold, 20)
    CONFIG_GLOBAL_PROPERTY(bool, actionOnClick, false)
    CONFIG_PROPERTY(int, groupPreviewNum, 3)
    CONFIG_PROPERTY(bool, openExpanded, false)
    CONFIG_GLOBAL_PROPERTY(QString, position, QStringLiteral("auto"))
    CONFIG_GLOBAL_PROPERTY(int, maxPopups, 8)
    CONFIG_GLOBAL_PROPERTY(int, maxNotifs, 50)
};

} // namespace caelestia::config
