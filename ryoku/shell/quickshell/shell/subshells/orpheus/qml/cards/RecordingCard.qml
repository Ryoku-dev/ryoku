pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function targetText() {
        if (args && args.target)
            return String(args.target);
        if (Recorder.regionGeom.length > 0)
            return "Region " + Recorder.regionGeom;
        if (scene && scene.screenName.length > 0)
            return "Display " + scene.screenName;
        return "Full display";
    }

    Connections {
        target: Recorder
        function onActiveChanged() {
            if (!Recorder.active && root.shown && root.scene)
                root.scene.closeCard();
        }
    }

    K.Sheet {
        id: sheet
        pageWidth: 300

        Column {
            width: parent.width
            spacing: K.Style.s2

            Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: K.Style.s2

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 6
                    height: 6
                    radius: 3
                    color: K.Style.alert
                }

                K.Label {
                    text: Recorder.elapsedText
                    role: "headline"
                    numeric: true
                }
            }

            K.Label {
                width: parent.width
                text: root.targetText()
                role: "body"
                tone: "dim"
                horizontalAlignment: Text.AlignHCenter
            }
        }

        K.HoldButton {
            anchors.horizontalCenter: parent.horizontalCenter
            width: parent.width
            height: K.Style.rowH
            text: "Stop recording"
            icon: "stop"
            danger: true
            enabled: Recorder.active
            onActivated: {
                Recorder.stop();
                if (root.scene)
                    root.scene.closeCard();
            }
        }
    }
}
