import QtQuick
import QtQuick.Effects
import "../"

Item {
    id: root

    property string agentId: ""
    readonly property url source: AgentsFormat.markSource(agentId)
    readonly property bool tinted: AgentsFormat.markTinted(agentId)

    Image {
        id: logo
        anchors.fill: parent
        source: root.source
        sourceSize: Qt.size(root.width * 2, root.height * 2)
        fillMode: Image.PreserveAspectFit
        smooth: true
        visible: root.source !== "" && !root.tinted
    }

    MultiEffect {
        anchors.fill: logo
        source: logo
        visible: root.source !== "" && root.tinted
        colorization: 1.0
        colorizationColor: ThemeBackend.text
    }

    Text {
        anchors.centerIn: parent
        visible: root.source === ""
        text: AgentsFormat.agentName(root.agentId).charAt(0)
        font.family: ThemeBackend.fontFamily
        font.pixelSize: root.height * 0.75
        font.weight: Font.Black
        color: ThemeBackend.text
    }
}
