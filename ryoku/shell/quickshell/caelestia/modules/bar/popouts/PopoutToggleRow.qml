pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls

RowLayout {
    id: root

    required property string label
    property alias checked: toggle.checked
    property alias toggle: toggle
    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property real rightMargin: Tokens.padding.extraSmall * scaleOffset

    Layout.fillWidth: true
    Layout.rightMargin: root.rightMargin
    spacing: Tokens.spacing.medium * root.scaleOffset

    StyledText {
        Layout.fillWidth: true
        text: root.label
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    StyledSwitch {
        id: toggle
    }
}
