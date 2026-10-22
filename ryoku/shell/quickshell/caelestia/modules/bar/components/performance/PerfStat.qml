import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

StyledRect {
    id: root

    required property string icon
    required property string valueText
    property color accent: Colours.palette.m3primary
    property real value: NaN
    property color textColor: Colours.palette.m3onSurface
    property color iconColor: accent
    property real widthFactor: 2.35
    property string maxText: "100"
    property bool showText: typeof Config.bar.performance !== "undefined" && Config.bar.performance.showText !== undefined ? Config.bar.performance.showText : true

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)
    readonly property int baseThickness: Tokens.sizes.bar.innerWidth
    readonly property int effectiveThickness: rawScale < 1.0 ? baseThickness : barThickness
    readonly property int iconSize: Math.round(effectiveThickness * 0.42)
    readonly property int textSize: Math.round(effectiveThickness * 0.32)
    readonly property real progress: isNaN(value) ? 0 : Math.max(0, Math.min(1, value))
    readonly property int hPadding: Tokens.padding.medium
    readonly property int vPadding: Tokens.padding.extraSmall
    readonly property int trackThickness: Math.max(4, Math.round(Tokens.padding.extraSmall * 0.8))
    readonly property int trackInset: Tokens.padding.medium

    color: Colours.tPalette.m3surfaceContainerHigh
    radius: Tokens.rounding.full
    clip: true

    implicitWidth: isHorizontal ? Math.max(contentRow.implicitWidth + hPadding * 2, Math.round(barThickness * widthFactor)) : barThickness
    implicitHeight: isHorizontal ? barThickness : Math.max(verticalCol.implicitHeight + vPadding * 2, Math.round(barThickness * widthFactor))

    StyledText {
        id: dummyText

        text: root.maxText
        font: Tokens.font.body.builders.small.size(root.textSize).weight(Font.DemiBold).build()
        visible: false
    }

    Item {
        id: progressTrack

        anchors.left: parent.left
        anchors.bottom: parent.bottom
        anchors.top: root.isHorizontal ? parent.top : undefined
        anchors.right: root.isHorizontal ? undefined : parent.right

        width: root.isHorizontal ? parent.width * root.progress : parent.width
        height: root.isHorizontal ? parent.height : parent.height * root.progress
        clip: true

        Behavior on width {
            Anim {
                type: Anim.FastSpatial
            }
        }

        Behavior on height {
            Anim {
                type: Anim.FastSpatial
            }
        }

        StyledRect {
            id: progressFill

            anchors.left: parent.left
            anchors.bottom: parent.bottom
            width: root.width
            height: root.height
            color: Qt.alpha(root.accent, 0.25)
            radius: root.radius
        }
    }

    RowLayout {
        id: contentRow

        anchors.fill: parent
        anchors.leftMargin: root.hPadding
        anchors.rightMargin: root.hPadding
        visible: root.isHorizontal
        spacing: Tokens.spacing.small

        Item {
            Layout.fillWidth: true
        }

        MaterialIcon {
            Layout.alignment: Qt.AlignVCenter
            fontStyle: Tokens.font.icon.builders.small.size(root.iconSize).build()
            text: root.icon
            color: root.iconColor
            fill: 1
        }

        StyledText {
            Layout.alignment: Qt.AlignVCenter
            Layout.preferredWidth: root.showText ? dummyText.implicitWidth : 0
            horizontalAlignment: Text.AlignHCenter
            text: root.valueText
            color: root.textColor
            font: Tokens.font.body.builders.small.size(root.textSize).weight(Font.DemiBold).build()
            elide: Text.ElideNone
            maximumLineCount: 1
            visible: root.showText
        }

        Item {
            Layout.fillWidth: true
        }
    }

    ColumnLayout {
        id: verticalCol

        anchors.centerIn: parent
        visible: !root.isHorizontal
        spacing: Tokens.spacing.extraSmall

        MaterialIcon {
            Layout.alignment: Qt.AlignHCenter
            fontStyle: Tokens.font.icon.builders.small.size(root.iconSize).build()
            text: root.icon
            color: root.iconColor
            fill: 1
        }

        StyledText {
            Layout.alignment: Qt.AlignHCenter
            horizontalAlignment: Text.AlignHCenter
            text: root.valueText
            color: root.textColor
            font: Tokens.font.body.builders.small.size(root.textSize).weight(Font.DemiBold).build()
            visible: root.showText
        }
    }
}
