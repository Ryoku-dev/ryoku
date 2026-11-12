import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: devicesView

    property var controller
    property real reveal: 1

    readonly property var report: controller ? controller.report : null
    readonly property var devices: report && Array.isArray(report.devices) ? report.devices : []
    readonly property bool providerAvailable: report && report.available !== false
    readonly property bool enabled: report && report.enabled === true

    implicitHeight: col.implicitHeight

    Column {
        id: col
        width: devicesView.width
        spacing: 18 * Theme.scale

        Text {
            width: parent.width
            visible: !devicesView.report
            text: I18n.tr("Checking saved and connected lighting devices…")
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.56 * devicesView.reveal)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            width: parent.width
            visible: devicesView.report && !devicesView.providerAvailable
            text: I18n.tr("OpenRGB is not installed. Run ryoku doctor, or open Ryoku Settings > Updates, to install the default app set.")
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.tertiary, 0.86 * devicesView.reveal)
            lineHeight: 1.4
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            width: parent.width
            visible: devicesView.providerAvailable && !devicesView.enabled
            text: I18n.tr("Turn lighting on in RGB lighting before adopting or changing a device.")
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.56 * devicesView.reveal)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            width: parent.width
            visible: devicesView.enabled && devicesView.controller.busy
                && devicesView.controller.operation === "scan" && devicesView.devices.length === 0
            text: I18n.tr("Detecting devices… the first OpenRGB start can take up to a minute.")
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.72 * devicesView.reveal)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Repeater {
            model: devicesView.enabled && devicesView.controller ? devicesView.controller.deviceKeys : []
            delegate: LightingDeviceCard {
                required property string modelData
                width: col.width
                device: devicesView.controller.devicesByKey[modelData] || ({ key: modelData })
                controller: devicesView.controller
                reveal: devicesView.reveal
            }
        }

        Rectangle {
            width: parent.width
            visible: devicesView.enabled && devicesView.devices.length === 0
                && !(devicesView.controller.busy && devicesView.controller.operation === "scan")
            height: emptyText.implicitHeight + 26 * Theme.scale
            radius: Theme.radius
            color: Theme.withAlpha(Theme.surfaceContainer, 0.5)
            border.width: 1
            border.color: Theme.withAlpha(Theme.outline, 0.3)

            Text {
                id: emptyText
                anchors.fill: parent
                anchors.margins: 13 * Theme.scale
                text: I18n.tr("No lighting device answered. OpenRGB reaches USB keyboards, mice and headsets plus SMBus RAM and motherboard lighting; Ryoku already loads i2c-dev. Supported ASUS laptop keyboards use asusd, and QMK/VIA boards use qmk_hid. Reconnect the device, then choose Detect devices in RGB lighting.")
                font.family: Theme.sans
                font.weight: Font.Normal
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.62 * devicesView.reveal)
                lineHeight: 1.42
                wrapMode: Text.WordWrap
                verticalAlignment: Text.AlignVCenter
                renderType: Text.NativeRendering
            }
        }

        Text {
            width: parent.width
            visible: devicesView.enabled && devicesView.devices.length > 0
            text: I18n.tr("Offline cards are saved devices, not false detections. Their choices stay ready for the next time they connect.")
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontFine
            color: Theme.withAlpha(Theme.surfaceText, 0.46 * devicesView.reveal)
            lineHeight: 1.35
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            width: parent.width
            visible: devicesView.controller && devicesView.controller.error.length > 0
            text: I18n.tr(devicesView.controller ? devicesView.controller.error : "")
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.tertiary, 0.88 * devicesView.reveal)
            lineHeight: 1.38
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }
}
