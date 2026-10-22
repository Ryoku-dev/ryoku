#pragma once

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class AudioSounds : public settings::ObjectNode {
    CONFIG_NODE(AudioSounds, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(bool, enabled, true)
    CONFIG_GLOBAL_PROPERTY(bool, cameraClick, true)
    CONFIG_GLOBAL_PROPERTY(bool, chargingStarted, true)
    CONFIG_GLOBAL_PROPERTY(bool, effectTick, true)
    CONFIG_GLOBAL_PROPERTY(bool, lock, true)
    CONFIG_GLOBAL_PROPERTY(bool, unlock, true)
    CONFIG_GLOBAL_PROPERTY(bool, lowBattery, true)
    CONFIG_GLOBAL_PROPERTY(bool, screenRecord, true)
    CONFIG_GLOBAL_PROPERTY(QString, notificationSound, QStringLiteral("Iapetus.wav"))
    CONFIG_GLOBAL_PROPERTY(QStringList, disabledNotifApps, QStringList())
    CONFIG_GLOBAL_PROPERTY(qreal, sfxVolume, 1.0)
    CONFIG_GLOBAL_PROPERTY(qreal, notificationVolume, 1.0)
};

class AudioConfig : public settings::ObjectNode {
    CONFIG_NODE(AudioConfig, settings::ObjectNode)

    CONFIG_SUBOBJECT(AudioSounds, sounds)
};

} // namespace caelestia::config
