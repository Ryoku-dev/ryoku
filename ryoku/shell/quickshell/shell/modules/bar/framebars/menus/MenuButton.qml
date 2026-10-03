pragma ComponentBehavior: Bound

import QtQuick
import shell.services

// The shared surface tile used by device entries and action rows: the reference
// `.ok-button-surface` and its size/state variants (contract 16 sec 2.2/2.6).
// A rounded surface fill that shifts to an
// 8% on-surface hover overlay, to the primary accent when `selected`, and dims
// its content to 38% when disabled unless `keepEnabledLook` holds it (the
// reference `.ok-button-no-disabled`).
//
// Size is minW x minH; a content-height row (a revealer header, a device entry)
// sets minH from its content's implicit height. The 8px inner padding is a
// filled `hold` the content anchors into, so nothing sizes off childrenRect
// (which would loop against a centred child).
Item {
    id: root

    property bool selected: false
    property bool keepEnabledLook: false
    property real radius: Theme.radiusWidget
    property real pad: Theme.paddingMd
    property real minW: 0
    property real minH: 0
    // The content colour a child icon/label binds to, resolved through the
    // state so a caller never recomputes surface/primary/disabled tones.
    readonly property color restingInk: Theme.ink(Theme.effectiveSurface)
    readonly property color contentColor: root.selected ? Theme.inkOn(Theme.primary, Theme.onPrimary)
        : (!root.enabled && !root.keepEnabledLook) ? Qt.rgba(root.restingInk.r, root.restingInk.g, root.restingInk.b, 0.38)
        : root.restingInk

    signal clicked()

    default property alias content: hold.data

    implicitWidth: root.minW
    implicitHeight: root.minH

    Rectangle {
        id: fill
        anchors.fill: parent
        radius: root.radius
        color: root.selected ? Theme.primary
            : (hover.hovered && root.enabled) ? Qt.tint(Theme.surface, Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08))
            : Theme.surface
        Behavior on color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }
    }

    Item {
        id: hold
        anchors.fill: parent
        anchors.margins: root.pad
    }

    HoverHandler { id: hover; enabled: root.enabled; cursorShape: Qt.PointingHandCursor }
    TapHandler { enabled: root.enabled; onTapped: root.clicked() }
}
