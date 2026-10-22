pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

ColumnLayout {
    id: root

    required property PopoutState popouts

    property bool _isSidebarOpen: false

    property real scaleOffset: 1.0
    property real fontScale: 1.0

    implicitWidth: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    spacing: Tokens.spacing.medium * scaleOffset

    StyledText {
        Layout.topMargin: Tokens.padding.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("Night Light")
        font.weight: 500
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    IconTextButton {
        Layout.fillWidth: true
        Layout.topMargin: Tokens.spacing.small * root.scaleOffset
        inactiveColour: HyprSunset.autoMode ? Colours.palette.m3primaryContainer : Colours.palette.m3surfaceVariant
        inactiveOnColour: HyprSunset.autoMode ? Colours.palette.m3onPrimaryContainer : Colours.palette.m3onSurfaceVariant
        verticalPadding: Tokens.padding.small * root.scaleOffset
        text: HyprSunset.autoMode ? qsTr("Auto") : qsTr("Manual")
        icon: "routine"
        
        onClicked: {
            HyprSunset.toggleAutoMode();
        }
    }

    StyledText {
        visible: HyprSunset.autoMode
        Layout.topMargin: Tokens.spacing.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("Daylight Temperature (%1K)").arg(Math.round(2000 + daySlider.pos * 4500))
        font.weight: Font.Medium
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
        font.features: { "tnum": 1 }
    }

    CustomMouseArea {
        visible: HyprSunset.autoMode
        Layout.fillWidth: true
        implicitHeight: Tokens.padding.medium * 3 * root.scaleOffset

        onWheel: event => {
            if (event.angleDelta.y > 0)
                HyprSunset.setDayTemperature(Math.min(6500, HyprSunset.dayTemperature + 100));
            else if (event.angleDelta.y < 0)
                HyprSunset.setDayTemperature(Math.max(2000, HyprSunset.dayTemperature - 100));
        }

        StyledSlider {
            id: daySlider

            anchors.left: parent.left
            anchors.right: parent.right
            implicitHeight: parent.implicitHeight

            value: Math.max(0, Math.min(1, (HyprSunset.dayTemperature - 2000) / 4500))
            onInteraction: v => HyprSunset.previewTemperature(Math.round(2000 + v * 4500))
            onReleased: v => {
                HyprSunset.stopPreview();
                HyprSunset.setDayTemperature(Math.round(2000 + v * 4500));
            }
        }
    }

    StyledText {
        Layout.topMargin: Tokens.spacing.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: HyprSunset.autoMode ? qsTr("Nightlight Temperature (%1K)").arg(Math.round(2000 + nightSlider.pos * 4500)) : qsTr("Temperature (%1K)").arg(Math.round(2000 + nightSlider.pos * 4500))
        font.weight: Font.Medium
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
        font.features: { "tnum": 1 }
    }

    CustomMouseArea {
        Layout.fillWidth: true
        implicitHeight: Tokens.padding.medium * 3 * root.scaleOffset

        onWheel: event => {
            if (event.angleDelta.y > 0)
                HyprSunset.setNightTemperature(Math.min(6500, HyprSunset.nightTemperature + 100));
            else if (event.angleDelta.y < 0)
                HyprSunset.setNightTemperature(Math.max(2000, HyprSunset.nightTemperature - 100));
        }

        StyledSlider {
            id: nightSlider

            anchors.left: parent.left
            anchors.right: parent.right
            implicitHeight: parent.implicitHeight

            value: Math.max(0, Math.min(1, (HyprSunset.nightTemperature - 2000) / 4500))
            onInteraction: v => HyprSunset.previewTemperature(Math.round(2000 + v * 4500))
            onReleased: v => {
                HyprSunset.stopPreview();
                HyprSunset.setNightTemperature(Math.round(2000 + v * 4500));
            }
        }
    }
}
