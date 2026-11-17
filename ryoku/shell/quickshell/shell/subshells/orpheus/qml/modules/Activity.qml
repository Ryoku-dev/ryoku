pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    readonly property var captureStreams: Audio.captureStreams || []
    readonly property var captureNames: {
        const names = [];
        const seen = ({});
        for (let i = 0; i < root.captureStreams.length; ++i) {
            const name = String(Audio.streamName(root.captureStreams[i]) || "Application");
            if (seen[name] !== true) {
                seen[name] = true;
                names.push(name);
            }
        }
        return names;
    }
    readonly property bool microphoneActive: captureNames.length > 0

    property bool present: microphoneActive

    implicitWidth: present ? activities.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Row {
        id: activities
        anchors.centerIn: parent
        spacing: Math.round(6 * K.Style.scale)

        Item {
            id: microphone
            visible: root.microphoneActive
            width: root.microphoneActive ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                scale: microphonePress.pressScale
                name: "mic"
                size: K.Style.barIcon
                tone: "alert"
                filled: true
            }

            K.Press {
                id: microphonePress
                radius: K.Style.radius
                onClicked: root.scene.toggleCard("controls", root, { page: "sound" })
                onRightClicked: Toggles.toggleMic()
                onHoveredChanged: root.scene.hoverCard("controls", root, hovered)
            }

            K.Hint {
                target: microphone
                text: "Microphone in use, " + root.captureNames.join(", ")
                shown: microphonePress.hovered
            }
        }
    }
}