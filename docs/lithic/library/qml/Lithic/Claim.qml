import QtQuick

Row {
    id: root

    property string tone: "idle"
    property string text: tone === "idle"
        ? "Idle"
        : tone.charAt(0).toUpperCase() + tone.slice(1)
    property real progress: tone === "becoming" ? 1 : tone === "idle" ? 0 : 1

    spacing: LithicTokens.space.s2

    Flake {
        anchors.verticalCenter: parent.verticalCenter
        tone: root.tone
        fillFraction: root.progress
    }

    Label {
        anchors.verticalCenter: parent.verticalCenter
        text: root.text
        typeRole: "label"
        ink: root.tone === "idle" ? Lithic.fg2 : Lithic.stone(root.tone)
    }
}
