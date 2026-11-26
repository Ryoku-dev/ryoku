import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// A control on an island
Item {
    id: btn

    property string glyph: ""
    property string label: ""
    property bool labelWhenOn: false
    property bool on: false
    property string tooltip: ""
    property int badge: 0
    property real size: 40
    property bool dropdown: false
    property bool expanded: false
    property bool bare: false
    property bool soon: false

    signal triggered()

    readonly property bool hovered: area.containsMouse
    readonly property bool _pressed: area.pressed
    readonly property bool _showLabel: btn.label.length > 0 && (!btn.labelWhenOn || btn.on)
    readonly property real _labelW: btn._showLabel ? labelText.implicitWidth + 8 * Theme.scale : 0

    implicitHeight: btn.size * Theme.scale
    implicitWidth: Math.max(btn.size * Theme.scale, face.implicitWidth + 22 * Theme.scale)
    Behavior on implicitWidth { enabled: !Lithic.reduceMotion; NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }

    CutRect {
        visible: !btn.bare
        anchors.fill: parent
        cut: (LithicTokens.cut.small + (btn._pressed ? Lithic.pressCutGrow : 0)) * Theme.scale
        fill: btn.on ? Lithic.container
            : btn._pressed ? Lithic.alpha(Lithic.fg, 0.14)
            : btn.hovered ? Lithic.alpha(Lithic.fg, 0.08)
            : Lithic.alpha(Lithic.fg, 0)
        Behavior on fill { ColorAnimation { duration: Theme.fast } }
        Behavior on cut { NumberAnimation { duration: 90 } }
    }

    Row {
        id: face
        visible: !btn.bare
        opacity: btn.soon ? 0.55 : 1
        anchors.centerIn: parent
        anchors.verticalCenterOffset: btn._pressed ? 1 : 0
        spacing: 0
        Glyph {
            visible: btn.glyph.length > 0
            anchors.verticalCenter: parent.verticalCenter
            name: btn.glyph
            size: 20
            ink: btn.on ? Lithic.containerInk : btn.hovered ? Lithic.fg : Lithic.fg2
            Behavior on ink { ColorAnimation { duration: Theme.fast } }
        }
        Item {
            width: btn._labelW + (btn.badge > 0 && btn._showLabel ? 6 * Theme.scale : 0)
            height: labelText.implicitHeight
            anchors.verticalCenter: parent.verticalCenter
            clip: true
            Behavior on width { enabled: !Lithic.reduceMotion; NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
            Text {
                id: labelText
                x: 8 * Theme.scale
                text: btn.label
                opacity: btn._showLabel ? 1 : 0
                font.family: Lithic.font.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fs(14)
                color: btn.on ? Lithic.containerInk : btn.hovered ? Lithic.fg : Lithic.fg2
                renderType: Text.NativeRendering
                Behavior on opacity { NumberAnimation { duration: Theme.standard } }
            }
        }
        CutRect {
            visible: btn.badge > 0 && btn._showLabel
            anchors.verticalCenter: parent.verticalCenter
            width: visible ? Math.max(18 * Theme.scale, inlineCount.implicitWidth + 10 * Theme.scale) : 0
            height: 18 * Theme.scale
            cut: 3 * Theme.scale
            fill: btn.on ? Lithic.alpha(Lithic.containerInk, 0.16) : Lithic.alpha(Lithic.sel, 0.18)
            Text {
                id: inlineCount
                anchors.centerIn: parent
                text: String(btn.badge)
                font.family: Lithic.font.mono
                font.pixelSize: Theme.fs(9.5)
                color: btn.on ? Lithic.containerInk : Lithic.sel
                renderType: Text.NativeRendering
            }
        }
        Text {
            visible: btn.soon
            anchors.verticalCenter: parent.verticalCenter
            leftPadding: 6 * Theme.scale
            text: I18n.tr("Soon")
            font.family: Lithic.font.mono
            font.pixelSize: Theme.fs(9)
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1
            color: Lithic.fg3
            renderType: Text.NativeRendering
        }
        Glyph {
            visible: btn.dropdown
            anchors.verticalCenter: parent.verticalCenter
            name: "expand_more"
            size: 16
            rotation: btn.expanded ? 180 : 0
            ink: btn.on ? Lithic.containerInk : Lithic.fg3
            Behavior on rotation { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
        }
    }

    CutRect {
        visible: btn.badge > 0 && !btn._showLabel
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.rightMargin: 3 * Theme.scale
        anchors.topMargin: 3 * Theme.scale
        width: Math.max(16 * Theme.scale, badgeText.implicitWidth + 8 * Theme.scale)
        height: 16 * Theme.scale
        cut: 3 * Theme.scale
        fill: Lithic.sel
        scale: btn.badge > 0 ? 1 : 0.4
        Behavior on scale { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack; easing.overshoot: 2 } }
        Text {
            id: badgeText
            anchors.centerIn: parent
            text: String(btn.badge)
            font.family: Lithic.font.mono
            font.pixelSize: Theme.fs(9)
            font.weight: Font.Bold
            color: Lithic.onSel
            renderType: Text.NativeRendering
        }
    }

    MouseArea {
        id: area
        z: 10
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: btn.soon ? Qt.ArrowCursor : Qt.PointingHandCursor
        onClicked: if (!btn.soon) btn.triggered()
        onEntered: if (btn.tooltip.length > 0) tipTimer.restart()
    }
    Timer { id: tipTimer; interval: 550 }

    CutRect {
        id: tip
        visible: opacity > 0.01
        opacity: btn.hovered && btn.tooltip.length > 0 && !tipTimer.running && !btn.expanded && !btn.on ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Theme.fast } }
        z: 60
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.bottom
        anchors.topMargin: (12 + (1 - tip.opacity) * -4) * Theme.scale
        width: tipText.implicitWidth + 18 * Theme.scale
        height: tipText.implicitHeight + 12 * Theme.scale
        cut: LithicTokens.cut.tooltip * Theme.scale
        fill: Lithic.fg
        Text {
            id: tipText
            anchors.centerIn: parent
            text: btn.tooltip
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(12)
            color: Lithic.ground
            renderType: Text.NativeRendering
        }
    }
}
