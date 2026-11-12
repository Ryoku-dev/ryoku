import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: row

    property var control: ({})
    property var host
    property real reveal: 1
    readonly property var controller: host ? host.lightingController : null
    readonly property bool bridgeAvailable: controller && controller.hubAvailable

    implicitHeight: col.implicitHeight

    Column {
        id: col
        width: row.width
        spacing: 12 * Theme.scale

        Text {
            width: parent.width
            visible: !row.bridgeAvailable
            text: I18n.tr("Lighting controls need the Ryoku Settings backend. Run ryoku doctor, or open Ryoku Settings > Updates, to restore the default app set.")
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.tertiary, 0.86 * row.reveal)
            lineHeight: 1.4
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Loader {
            width: parent.width
            active: row.bridgeAvailable
            sourceComponent: row.control && row.control.part === "devices" ? devicesComponent : overviewComponent
        }
    }

    Component {
        id: overviewComponent
        LightingOverview {
            width: col.width
            controller: row.controller
            reveal: row.reveal
        }
    }

    Component {
        id: devicesComponent
        LightingDevices {
            width: col.width
            controller: row.controller
            reveal: row.reveal
        }
    }
}
