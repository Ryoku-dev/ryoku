import QtQuick
import Lithic

Item {
    id: sw

    property bool checked: false

    signal toggled(bool checked)

    implicitWidth: 44 * Theme.scale
    implicitHeight: 24 * Theme.scale

    readonly property bool hovered: area.containsMouse
    readonly property real _pad: 4 * Theme.scale
    readonly property real _knob: height - sw._pad * 2

    CutRect {
        id: track
        anchors.fill: parent
        cut: LithicTokens.cut.toggleTrack * Theme.scale
        fill: sw.checked ? Theme.withAlpha(Lithic.sel, sw.enabled ? 1 : 0.45)
             : sw.hovered && sw.enabled ? Lithic.alpha(Lithic.fg, 0.14)
             : Lithic.field
        strokeWidth: sw.checked ? 0 : 1
        strokeColor: Lithic.alpha(Lithic.fg, sw.enabled ? 0.28 : 0.14)
        Behavior on fill { ColorAnimation { duration: Theme.fast } }
    }

    CutRect {
        id: knob
        width: sw.checked ? sw._knob + 2 * Theme.scale : sw._knob - 2 * Theme.scale
        height: width
        y: (sw.height - height) / 2
        x: sw.checked ? sw.width - width - sw._pad : sw._pad + 1
        cut: LithicTokens.cut.toggleKnob * Theme.scale
        fill: sw.checked ? Lithic.onSel : Lithic.alpha(Lithic.fg, sw.enabled ? 0.62 : 0.3)
        Behavior on x { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack; easing.overshoot: 1.6 } }
        Behavior on width { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
        Behavior on fill { ColorAnimation { duration: Theme.fast } }
    }

    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        enabled: sw.enabled
        cursorShape: Qt.PointingHandCursor
        onClicked: sw.toggled(!sw.checked)
    }
}
