import QtQuick
import Lithic

// moved fires while dragging; released fires once on let-go.
Item {
    id: slider

    property real from: 0
    property real to: 1
    property real value: 0
    // Snap increment; 0 leaves the value continuous.
    property real step: 0

    readonly property bool dragging: drag.pressed
    property bool focused: false

    signal moved(real value)
    signal released(real value)

    implicitHeight: 26 * Theme.scale
    implicitWidth: 160 * Theme.scale

    readonly property real _knob: 12 * Theme.scale
    readonly property real _inset: slider._knob * 0.5
    readonly property real _trackW: Math.max(1, width - slider._inset * 2)
    readonly property real _span: (slider.to - slider.from) || 1
    readonly property real _t: Math.max(0, Math.min(1, (slider.value - slider.from) / slider._span))
    readonly property real _knobX: slider._inset + slider._t * slider._trackW

    function _snap(v) {
        var c = Math.max(slider.from, Math.min(slider.to, v))
        if (slider.step > 0)
            c = slider.from + Math.round((c - slider.from) / slider.step) * slider.step
        return c
    }
    function _fromX(x) {
        var t = (x - slider._inset) / slider._trackW
        return slider._snap(slider.from + t * slider._span)
    }

    CutRect {
        height: 6 * Theme.scale
        width: slider._trackW
        x: slider._inset
        y: (slider.height - height) / 2
        cut: 2 * Theme.scale
        fill: Lithic.alpha(Lithic.fg, slider.enabled ? 0.12 : 0.06)
    }

    CutRect {
        height: 6 * Theme.scale
        width: Math.max(0, slider._knobX - slider._inset)
        x: slider._inset
        y: (slider.height - height) / 2
        cut: 2 * Theme.scale
        fill: slider.enabled ? Lithic.sel : Lithic.alpha(Lithic.fg, 0.3)
    }

    CutRect {
        width: slider._knob + 4 * Theme.scale
        height: width
        x: slider._knobX - width / 2
        y: (slider.height - height) / 2
        cut: 4 * Theme.scale
        fill: slider.enabled ? Lithic.sel : Lithic.alpha(Lithic.fg, 0.4)
        strokeWidth: 2 * Theme.scale
        strokeColor: Lithic.stone2
        scale: slider.dragging ? 1.25 : slider.focused ? 1.15 : 1
        Behavior on scale { NumberAnimation { duration: Theme.fast; easing.type: Easing.OutBack; easing.overshoot: 2.4 } }
    }

    MouseArea {
        id: drag
        anchors.fill: parent
        enabled: slider.enabled
        preventStealing: true
        cursorShape: Qt.PointingHandCursor
        onPressed: (m) => { var v = slider._fromX(m.x); if (v !== slider.value) { slider.value = v; slider.moved(v) } }
        onPositionChanged: (m) => { if (pressed) { var v = slider._fromX(m.x); if (v !== slider.value) { slider.value = v; slider.moved(v) } } }
        onReleased: slider.released(slider.value)
    }
}
