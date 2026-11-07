pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.iris.style
import inir.modules.iris.components

IrisSurface {
    required property var page
    id: card
    quiet: true
    radius: page.radius

    AiUsageGlassPane {
        anchors.fill: parent
        page: card.page
        z: -1
    }

    Rectangle {
        anchors.fill: parent
        radius: card.radius
        z: -1
        color: "transparent"
        border.width: 1
        border.color: ColorUtils.applyAlpha(IrisStyle.text, 0.10)
    }
}
