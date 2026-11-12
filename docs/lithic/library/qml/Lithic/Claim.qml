import QtQuick

Row {
    id: root

    property string tone: "idle"
    property string text: tone === "alive" ? Lithic.tr("Alive")
        : tone === "becoming" ? Lithic.tr("Becoming")
        : tone === "attention" ? Lithic.tr("Attention")
        : tone === "paused" ? Lithic.tr("Paused")
        : tone === "failed" ? Lithic.tr("Failed")
        : tone === "idle" ? Lithic.tr("Idle")
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
