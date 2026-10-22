pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.services

StyledRect {
    id: root

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * Math.max(0.6, !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0))

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.full

    implicitWidth: isHorizontal ? (layoutLabel.implicitWidth + Tokens.padding.medium * 2) : barThickness
    implicitHeight: isHorizontal ? barThickness : (layoutLabel.implicitHeight + Tokens.padding.medium * 2)

    StyledText {
        id: layoutLabel

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Tokens.padding.medium
        anchors.rightMargin: Tokens.padding.medium

        horizontalAlignment: Text.AlignHCenter
        text: Kwin.kbLayout.toUpperCase()
        color: Colours.palette.m3secondary
        font: Tokens.font.mono.medium
        elide: Text.ElideRight
        maximumLineCount: 1
    }
}