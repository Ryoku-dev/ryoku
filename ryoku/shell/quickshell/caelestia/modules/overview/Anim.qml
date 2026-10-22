pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config

QtObject {
    id: root

    property int baseDuration: GlobalConfig.overview.baseDuration
    property real blobScaleSpeed: GlobalConfig.overview.blobScaleSpeed
    property real wallpaperFadeSpeed: GlobalConfig.overview.wallpaperFadeSpeed
    property real gridFadeSpeed: GlobalConfig.overview.gridFadeSpeed
    property int easingType: GlobalConfig.overview.easingType
    readonly property int blobDuration: Math.round(baseDuration / blobScaleSpeed)
    readonly property int wallpaperDuration: Math.round(baseDuration / wallpaperFadeSpeed)
    readonly property int gridDuration: Math.round(baseDuration / gridFadeSpeed)
}
