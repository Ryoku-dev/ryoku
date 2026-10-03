pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.PluginKit.Singletons

// content/Widget.qml is the one view every host mounts (on the bar, this is the
// glyph). It reads live state from the service through pluginApi.mainInstance.
// The sidebar host sets open, reveal, tabActive, width, s and pluginApi; other
// hosts set active, density and widthBudget as well. Read host values, never
// assign them.
Item {
    id: root

    property var pluginApi
    property var screen
    property bool active: false
    property string density: "glyph"
    property real s: 1
    property real widthBudget: 0
    property bool open: false
    property real reveal: 0
    property bool tabActive: false
    signal requestClose()

    readonly property var service: pluginApi ? pluginApi.mainInstance : null
    readonly property int count: service ? service.count : 0

    implicitWidth: row.implicitWidth
    implicitHeight: Math.max(row.implicitHeight, 18 * root.s)

    Row {
        id: row
        anchors.verticalCenter: parent.verticalCenter
        spacing: 6 * root.s

        // The plugin's mark. Tint it with the themed accent so it follows the
        // active palette (Theme resolves colour from the system scheme).
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "\u25C6"
            color: (root.active || root.open) ? Theme.accent : Theme.dim
            font.family: Theme.mono
            font.pixelSize: 13 * root.s
        }

        // The label beside the mark, elided to the width the host allows.
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.count
            color: Theme.bright
            font.family: Theme.font
            font.pixelSize: 13 * root.s
            elide: Text.ElideRight
            width: root.widthBudget > 0 ? Math.min(implicitWidth, root.widthBudget) : implicitWidth
        }
    }

    MouseArea {
        anchors.fill: parent
        onClicked: {
            if (!root.pluginApi)
                return;
            if (root.open)
                root.requestClose();
            else if (root.pluginApi.togglePanel)
                root.pluginApi.togglePanel();
        }
    }
}
