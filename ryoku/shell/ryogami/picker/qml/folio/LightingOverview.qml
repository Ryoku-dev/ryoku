import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: overview

    property var controller
    property real reveal: 1

    readonly property var report: controller ? controller.report : null
    readonly property bool providerAvailable: report && report.available !== false
    readonly property bool enabled: report && report.enabled === true
    readonly property var devices: report && Array.isArray(report.devices) ? report.devices : []
    readonly property int onlineCount: {
        var count = 0
        for (var i = 0; i < overview.devices.length; ++i)
            if (overview.devices[i].online === true)
                ++count
        return count
    }
    readonly property int followingCount: {
        var count = 0
        for (var i = 0; i < overview.devices.length; ++i)
            if (overview.devices[i].online === true && overview.devices[i].following === true)
                ++count
        return count
    }
    readonly property int managedCount: {
        var count = 0
        for (var i = 0; i < overview.devices.length; ++i)
            if (overview.devices[i].managed === true)
                ++count
        return count
    }

    implicitHeight: col.implicitHeight

    function statusText() {
        if (!overview.report)
            return I18n.tr("Checking lighting support…")
        if (!overview.providerAvailable)
            return I18n.tr("OpenRGB is not installed. Run ryoku doctor, or open Ryoku Settings > Updates, to install the default app set.")
        if (overview.controller.busy && overview.controller.operation === "scan")
            return I18n.tr("Detecting devices… the first OpenRGB start can take up to a minute.")
        if (overview.controller.busy)
            return I18n.tr("Applying lighting changes…")
        if (!overview.enabled)
            return I18n.tr("Lighting is off. Ryoku does not scan or write to RGB hardware while it is off.")
        if (overview.onlineCount === 0)
            return I18n.tr("Lighting is on. Detect devices to look for connected hardware.")
        return I18n.tr("%1 devices connected · %2 following the palette").arg(overview.onlineCount).arg(overview.followingCount)
    }

    Column {
        id: col
        width: overview.width
        spacing: 14 * Theme.scale

        Item {
            width: parent.width
            height: Math.max(masterText.implicitHeight, masterSwitch.height)

            Column {
                id: masterText
                anchors.left: parent.left
                anchors.right: masterSwitch.left
                anchors.rightMargin: 16 * Theme.scale
                spacing: 4 * Theme.scale

                Text {
                    width: parent.width
                    text: I18n.tr("Let Ryoku control lighting")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.withAlpha(Theme.surfaceText, overview.providerAvailable ? overview.reveal : 0.4)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Your choices survive updates, shell reloads and reboots. Turning this off hands every adopted device back.")
                    font.family: Theme.sans
                    font.weight: Font.Normal
                    font.pixelSize: Theme.fontBase
                    color: Theme.withAlpha(Theme.surfaceText, 0.56 * overview.reveal)
                    lineHeight: 1.38
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
            }

            Switch {
                id: masterSwitch
                anchors.right: parent.right
                anchors.top: parent.top
                checked: overview.enabled
                enabled: overview.providerAvailable && !overview.controller.busy
                onToggled: (on) => overview.controller.setEnabled(on)
            }
        }

        FolioRule { width: parent.width; alpha: 0.34; reveal: overview.reveal }

        Row {
            width: parent.width
            spacing: 10 * Theme.scale

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 36 * Theme.scale
                height: 36 * Theme.scale
                radius: width / 2
                color: overview.report && overview.report.accent ? overview.report.accent : Theme.primary
                border.width: 2
                border.color: Theme.withAlpha(Theme.surfaceText, 0.65)
                Behavior on color { ColorAnimation { duration: Theme.fast } }
            }

            Column {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - 46 * Theme.scale
                spacing: 2 * Theme.scale
                Text {
                    width: parent.width
                    text: I18n.tr("Live palette accent")
                    font.family: Theme.sans
                    font.weight: Font.DemiBold
                    font.pixelSize: Theme.fontBody
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: overview.report && overview.report.accent ? String(overview.report.accent) : I18n.tr("Waiting for the palette")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontFine
                    color: Theme.withAlpha(Theme.surfaceText, 0.54)
                    renderType: Text.NativeRendering
                }
            }
        }

        Rectangle {
            width: parent.width
            height: statusLabel.implicitHeight + 18 * Theme.scale
            radius: Theme.radius
            color: Theme.withAlpha(Theme.surfaceContainer, 0.55)
            border.width: 1
            border.color: Theme.withAlpha(Theme.outline, 0.3)

            Text {
                id: statusLabel
                anchors.fill: parent
                anchors.margins: 9 * Theme.scale
                text: overview.statusText()
                font.family: Theme.sans
                font.weight: Font.Normal
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.7 * overview.reveal)
                wrapMode: Text.WordWrap
                verticalAlignment: Text.AlignVCenter
                renderType: Text.NativeRendering
            }
        }

        Flow {
            width: parent.width
            spacing: 8 * Theme.scale
            visible: overview.providerAvailable && overview.enabled

            FolioAction {
                label: overview.controller.busy && overview.controller.operation === "scan"
                    ? I18n.tr("Detecting…")
                    : (overview.controller.hasScanned || overview.devices.length > 0
                       ? I18n.tr("Rescan devices") : I18n.tr("Detect devices"))
                enabled: !overview.controller.busy
                onTriggered: overview.controller.scan()
            }
            FolioAction {
                label: I18n.tr("Follow on every device")
                enabled: !overview.controller.busy && overview.onlineCount > 0
                onTriggered: overview.controller.followAll(true)
            }
            FolioAction {
                label: I18n.tr("Hand back every device")
                enabled: !overview.controller.busy && overview.managedCount > 0
                confirm: true
                onTriggered: overview.controller.followAll(false)
            }
            FolioAction {
                label: I18n.tr("Reapply current look")
                enabled: !overview.controller.busy && overview.managedCount > 0
                onTriggered: overview.controller.apply()
            }
        }

        Text {
            width: parent.width
            visible: overview.controller && overview.controller.error.length > 0
            text: I18n.tr(overview.controller ? overview.controller.error : "")
            font.family: Theme.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.tertiary, 0.88 * overview.reveal)
            lineHeight: 1.38
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }
}
