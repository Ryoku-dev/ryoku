pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K

// The lead's actions as numbered tracks: the primary first with an Enter keycap,
// then the rest. The focused one wears the list's bead, which stretches along its
// travel and squashes on arrival the way the selection does in the track list.
Item {
    id: root

    // [{ label, meta, glyph, primary }]
    property var items: []
    // the focused entry, -1 while the keyboard is in the track list
    property int focusIndex: -1
    property bool numbered: true
    property int numberFrom: 1
    // the lead's scroller, kept in step with the focused entry
    property var scroller: null

    readonly property real rowH: 36
    readonly property bool micro: K.Look.microMotion && !K.Style.still

    signal activated(int index)

    implicitWidth: 240
    implicitHeight: root.items.length * root.rowH

    function numeral(index) {
        const value = root.numberFrom + index;
        if (K.Look.lithic)
            return value < 10 ? "0" + value : String(value);
        return String(value);
    }

    function reveal() {
        if (!root.scroller || root.focusIndex < 0)
            return;
        const top = root.mapToItem(root.scroller.contentItem, 0, root.focusIndex * root.rowH).y;
        const bottom = top + root.rowH;
        if (top < root.scroller.contentY)
            root.scroller.contentY = Math.max(0, top);
        else if (bottom > root.scroller.contentY + root.scroller.height)
            root.scroller.contentY = bottom - root.scroller.height;
    }

    onFocusIndexChanged: {
        bead.follow();
        Qt.callLater(root.reveal);
    }

    Component.onCompleted: bead.follow()

    K.Shape {
        id: bead

        readonly property real targetY: Math.max(0, root.focusIndex) * root.rowH
        property real travelY: 0
        property bool glide: false
        property bool travelling: false
        property real squash: 0
        readonly property real stretch: Math.min(root.rowH * 0.5, Math.abs(bead.travelY - bead.targetY) * 0.25)

        // computed here: the targetY binding can still hold the previous index while focusIndexChanged runs
        function follow() {
            const target = Math.max(0, root.focusIndex) * root.rowH;
            const wasShown = bead.opacity > 0.01;
            bead.glide = wasShown && root.micro && root.focusIndex >= 0;
            bead.travelling = bead.glide && Math.abs(bead.travelY - target) > 0.5;
            bead.travelY = target;
        }

        onTravelYChanged: {
            if (bead.travelling && Math.abs(bead.travelY - bead.targetY) < 0.5) {
                bead.travelling = false;
                squashPulse.restart();
            }
        }

        x: 0
        width: root.width
        y: bead.travelY - bead.stretch / 2 + bead.squash / 2
        height: root.rowH + bead.stretch - bead.squash
        opacity: root.focusIndex >= 0 && root.items.length > 0 ? 1 : 0
        cutSize: K.Look.material ? K.Look.cutXl : K.Look.adwaita ? K.Look.cutMd : K.Look.cutSm
        fill: K.Look.lithic ? K.Style.bone : K.Look.material ? K.Style.tint16 : K.Style.tint10
        strokeWidth: K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Style.lineSoft

        Behavior on travelY {
            enabled: bead.glide
            NumberAnimation {
                duration: K.Style.ms(K.Look.stateMs + 60)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Look.enterCurve
            }
        }
        Behavior on opacity {
            enabled: root.micro
            NumberAnimation { duration: K.Style.quick }
        }

        SequentialAnimation {
            id: squashPulse
            NumberAnimation {
                target: bead
                property: "squash"
                to: 3
                duration: K.Style.ms(45)
                easing.type: Easing.OutQuad
            }
            NumberAnimation {
                target: bead
                property: "squash"
                to: 0
                duration: K.Style.ms(45)
                easing.type: Easing.InOutQuad
            }
        }
    }

    Column {
        width: root.width

        Repeater {
            model: root.items

            delegate: Item {
                id: track

                required property var modelData
                required property int index
                readonly property bool focused: track.index === root.focusIndex
                readonly property bool onBone: track.focused && K.Look.lithic

                width: root.width
                height: root.rowH
                scale: trackPress.pressed && root.micro ? K.Look.pressScale : 1

                Behavior on scale {
                    enabled: root.micro
                    NumberAnimation { duration: K.Style.snap }
                }

                K.Shape {
                    anchors.fill: parent
                    visible: trackPress.hovered && !track.focused
                    cutSize: bead.cutSize
                    fill: K.Style.tint5
                }

                Item {
                    id: marker
                    anchors.left: parent.left
                    anchors.leftMargin: K.Style.s3
                    anchors.verticalCenter: parent.verticalCenter
                    width: root.numbered || String(track.modelData.glyph || "").length > 0 ? 24 : 0
                    height: parent.height

                    K.Label {
                        visible: root.numbered
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.numeral(track.index)
                        role: K.Look.lithic ? "micro" : "caption"
                        tone: track.onBone ? "onBone" : "faint"
                        numeric: true
                    }

                    K.Glyph {
                        visible: !root.numbered && String(track.modelData.glyph || "").length > 0
                        anchors.verticalCenter: parent.verticalCenter
                        name: String(track.modelData.glyph || "")
                        size: 18
                        tone: track.onBone ? "onBone" : "dim"
                    }
                }

                Row {
                    id: trailing
                    anchors.right: parent.right
                    anchors.rightMargin: K.Style.s3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: K.Style.s2

                    K.Label {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: text.length > 0
                        text: String(track.modelData.meta || "")
                        role: "caption"
                        tone: track.onBone ? "onBone" : "muted"
                        numeric: true
                    }

                    KeyCap {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: track.modelData.primary === true
                        text: "Enter"
                        inverse: track.onBone
                    }
                }

                K.Label {
                    anchors.left: marker.right
                    anchors.leftMargin: marker.width > 0 ? K.Style.s2 : 0
                    anchors.right: trailing.left
                    anchors.rightMargin: K.Style.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(track.modelData.label || "")
                    role: track.modelData.primary === true ? "bodyStrong" : "body"
                    tone: track.onBone ? "onBone" : track.focused || track.modelData.primary === true ? "ink" : "dim"
                }

                K.Press {
                    id: trackPress
                    tint: false
                    onClicked: root.activated(track.index)
                }
            }
        }
    }
}
