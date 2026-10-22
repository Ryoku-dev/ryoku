#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class LockConfig : public settings::ObjectNode {
    CONFIG_NODE(LockConfig, settings::ObjectNode)

    CONFIG_PROPERTY(bool, recolourLogo, true)
    CONFIG_GLOBAL_PROPERTY(bool, enableFprint, true)
    CONFIG_GLOBAL_PROPERTY(int, maxFprintTries, 3)
    CONFIG_GLOBAL_PROPERTY(bool, enableHowdy, true)
    CONFIG_GLOBAL_PROPERTY(int, maxHowdyTries, 3)
    CONFIG_GLOBAL_PROPERTY(bool, triggerHowdyOnWake, true)
    CONFIG_GLOBAL_PROPERTY(int, profilePicShape, 13)
    CONFIG_GLOBAL_PROPERTY(bool, rotateProfilePic, false)
    CONFIG_PROPERTY(bool, hideNotifs, false)
    CONFIG_GLOBAL_PROPERTY(bool, lockOnStartup, false)
    CONFIG_GLOBAL_PROPERTY(bool, syncWallpaper, true)
    CONFIG_GLOBAL_PROPERTY(bool, blurWallpaper, false)
    CONFIG_GLOBAL_PROPERTY(bool, showSleep, true)
    CONFIG_GLOBAL_PROPERTY(bool, showHibernate, false)
    CONFIG_GLOBAL_PROPERTY(bool, showSwitchUser, true)
    CONFIG_GLOBAL_PROPERTY(bool, showLogout, true)
    CONFIG_GLOBAL_PROPERTY(bool, showReboot, false)
    CONFIG_GLOBAL_PROPERTY(bool, showShutdown, false)
};

} // namespace caelestia::config
