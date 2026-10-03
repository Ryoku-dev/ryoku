pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    property string title: ""
    property string glyph: ""
    property string eyebrow: ""
    property int index: 0
    property bool open: false
    property real reveal: 0
    property bool tabActive: false
    default property alias content: contentColumn.data

    readonly property bool shouldEnter: root.tabActive && root.reveal > 0.01
    property bool entered: false

    implicitHeight: frame.implicitHeight
    height: implicitHeight
    opacity: entered ? 1 : 0
    transform: Translate {
        id: entranceTranslate
        y: root.entered ? 0 : 24
        Behavior on y {
            enabled: !Motion.reduce && !Tokens.reduceMotion
            NumberAnimation {
                duration: Math.round(Tokens.swap * SidebarState.motionMultiplier)
                easing.type: Tokens.ease
            }
        }
    }

    onShouldEnterChanged: {
        if (shouldEnter) {
            entered = false;
            entranceDelay.restart();
        } else {
            entranceDelay.stop();
            entered = false;
        }
    }
    Component.onCompleted: if (shouldEnter) entranceDelay.start()

    Timer {
        id: entranceDelay
        interval: (Motion.reduce || Tokens.reduceMotion)
            ? 0 : Math.round(root.index * 40 * SidebarState.motionMultiplier)
        onTriggered: root.entered = true
    }

    Behavior on opacity {
        enabled: !Motion.reduce && !Tokens.reduceMotion
        NumberAnimation {
            duration: Math.round(Tokens.swap * SidebarState.motionMultiplier)
            easing.type: Tokens.ease
        }
    }


    Rectangle {
        id: frame
        anchors { left: parent.left; right: parent.right; top: parent.top }
        implicitHeight: header.height + contentColumn.implicitHeight + Tokens.s4
        height: implicitHeight
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.line
        clip: true

        Grain { anchors.fill: parent }

        Item {
            id: header
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: (root.eyebrow !== "" ? Tokens.s6 : Tokens.s5) + Tokens.s4

            Text {
                id: glyphText
                anchors { left: parent.left; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                text: root.glyph
                visible: text !== ""
                color: Tokens.inkMuted
                font.family: "Material Symbols Rounded"
                font.pixelSize: Tokens.fValue
            }

            Column {
                anchors {
                    left: glyphText.visible ? glyphText.right : parent.left
                    leftMargin: Tokens.s3
                    right: parent.right
                    rightMargin: Tokens.s4
                    verticalCenter: parent.verticalCenter
                }
                spacing: 2

                Text {
                    width: parent.width
                    text: root.title
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    visible: root.eyebrow !== ""
                    text: root.eyebrow
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                    elide: Text.ElideRight
                }
            }
        }

        Column {
            id: contentColumn
            anchors {
                left: parent.left
                right: parent.right
                top: header.bottom
                leftMargin: Tokens.s4
                rightMargin: Tokens.s4
            }
            spacing: Tokens.s3
        }
    }

}
