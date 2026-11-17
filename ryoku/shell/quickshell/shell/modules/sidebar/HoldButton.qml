pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui.Singletons
import shell.services

QQC.AbstractButton {
    id: root

    required property real s
    property string glyph: ""
    property int holdDuration: 650
    property bool subtle: false
    property bool labelled: true
    // a whole sentence per action, so a language can word and case it its own way
    property string tip: ""
    signal held()

    QtObject {
        id: holdState
        property real progress: 0
        property real fillOpacity: 1
        property bool active: false
        property bool completed: false
        readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    }

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: Math.ceil(normalLabel.implicitWidth + (glyph !== "" ? (Tokens.s6 + Tokens.s1) * s : 0) + Tokens.s5 * s)
    implicitHeight: (Tokens.ctlH + Tokens.s3) * s
    Accessible.name: text

    function beginHold(): void {
        if (!root.enabled || holdState.active)
            return;
        holdFill.stop();
        drainFill.stop();
        flash.stop();
        reducedHold.stop();
        holdState.progress = 0;
        holdState.fillOpacity = 1;
        holdState.completed = false;
        holdState.active = true;
        if (holdState.motionAllowed)
            holdFill.start();
        else
            reducedHold.start();
    }

    function endHold(): void {
        if (!holdState.active)
            return;
        holdState.active = false;
        holdFill.stop();
        reducedHold.stop();
        if (holdState.completed)
            return;
        if (holdState.motionAllowed)
            drainFill.start();
        else
            holdState.progress = 0;
    }

    function completeHold(): void {
        if (!holdState.active || holdState.completed)
            return;
        holdState.completed = true;
        holdState.active = false;
        holdState.progress = 1;
        root.held();
        flash.restart();
    }

    onPressed: root.beginHold()
    onReleased: root.endHold()
    onCanceled: root.endHold()
    onEnabledChanged: if (!enabled) root.endHold()

    Keys.onPressed: function(event) {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.beginHold();
            event.accepted = true;
        }
    }
    Keys.onReleased: function(event) {
        if ((event.key === Qt.Key_Space || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !event.isAutoRepeat) {
            root.endHold();
            event.accepted = true;
        }
    }

    NumberAnimation {
        id: holdFill
        target: holdState
        property: "progress"
        to: 1
        duration: root.holdDuration
        easing.type: Easing.Linear
        onFinished: root.completeHold()
    }
    NumberAnimation {
        id: drainFill
        target: holdState
        property: "progress"
        to: 0
        duration: Tokens.move
        easing.type: Tokens.ease
    }
    Timer {
        id: reducedHold
        interval: root.holdDuration
        onTriggered: {
            holdState.progress = 1;
            root.completeHold();
        }
    }
    SequentialAnimation {
        id: flash
        NumberAnimation {
            target: holdState
            property: "fillOpacity"
            from: 1
            to: 0.5
            duration: holdState.motionAllowed ? Tokens.snap / 2 : 0
            easing.type: Tokens.easeSnap
        }
        NumberAnimation {
            target: holdState
            property: "fillOpacity"
            from: 0.5
            to: 1
            duration: holdState.motionAllowed ? Tokens.snap / 2 : 0
            easing.type: Tokens.easeSnap
        }
        ScriptAction { script: holdState.progress = 0 }
    }

    background: Rectangle {
        radius: Tokens.radius * root.s * 1.5
        color: root.down ? Tokens.tint16 : root.hovered ? Tokens.tint10 : root.subtle ? "transparent" : Tokens.tint5
        border.width: Tokens.border
        border.color: root.visualFocus ? Tokens.sun : Tokens.lineSoft
        clip: true

        Rectangle {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: parent.width * holdState.progress
            color: Tokens.bone
            opacity: holdState.fillOpacity
        }

        Behavior on color {
            enabled: holdState.motionAllowed
            ColorAnimation { duration: Tokens.snap }
        }
    }

    contentItem: Item {
        id: contentRoot
        implicitWidth: normalLabel.implicitWidth + (root.glyph !== "" ? (Tokens.s6 + Tokens.s1) * root.s : 0)

        Item {
            anchors.fill: parent

            Text {
                id: normalIcon
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: normalLabel.visible ? Tokens.s5 * root.s : parent.width
                visible: root.glyph !== ""
                text: root.glyph
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: Tokens.fBody * root.s + Tokens.s2 * root.s
                horizontalAlignment: Text.AlignHCenter
                Accessible.ignored: true
                scale: root.down || holdState.active ? 0.9 : 1
                Behavior on scale {
                    enabled: holdState.motionAllowed
                    NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                }
            }
            Text {
                id: normalLabel
                anchors.left: root.glyph !== "" ? normalIcon.right : parent.left
                anchors.leftMargin: root.glyph !== "" ? Tokens.s2 * root.s : 0
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: root.labelled && root.width >= implicitWidth + (root.glyph !== "" ? (Tokens.s6 + Tokens.s5) * root.s : Tokens.s5 * root.s)
                text: root.text
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                font.weight: Font.Medium
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
            }
        }

        Item {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: parent.width * holdState.progress
            opacity: holdState.fillOpacity
            clip: true

            Item {
                width: contentRoot.width
                height: contentRoot.height

                Text {
                    id: filledIcon
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: normalLabel.visible ? Tokens.s5 * root.s : parent.width
                    visible: root.glyph !== ""
                    text: root.glyph
                    color: Tokens.inkOnBone
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: Tokens.fBody * root.s + Tokens.s2 * root.s
                    horizontalAlignment: Text.AlignHCenter
                    Accessible.ignored: true
                    scale: root.down || holdState.active ? 0.9 : 1
                    Behavior on scale {
                        enabled: holdState.motionAllowed
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                    }
                }
                Text {
                    anchors.left: root.glyph !== "" ? filledIcon.right : parent.left
                    anchors.leftMargin: root.glyph !== "" ? Tokens.s2 * root.s : 0
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: normalLabel.visible
                    text: root.text
                    color: Tokens.inkOnBone
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                    elide: Text.ElideRight
                }
            }
        }
    }

    CornerTip { s: root.s; visible: root.tip !== "" && root.hovered && !root.down; text: root.tip }
    HoverHandler { cursorShape: Qt.PointingHandCursor }
}
