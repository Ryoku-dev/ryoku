pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell.Io
import inir.modules.common
import inir.modules.iris.components
import inir.modules.ii.overlay
import inir.services

OverlayBackground {
    id: root

    property string status: ""
    implicitWidth: 300
    implicitHeight: 110
    radius: OverlayLook.roundingNormal

    function applyLimit(): void {
        const value = Number(field.text)
        if (!Number.isInteger(value) || value < 0 || value > 1000) {
            root.status = Translation.tr("Enter a number from 0 to 1000")
            return
        }
        setter.command = ["bash", "-lc", "mkdir -p \"$HOME/.config/MangoHud\"; f=\"$HOME/.config/MangoHud/MangoHud.conf\"; touch \"$f\"; if grep -q '^fps_limit=' \"$f\"; then sed -i 's/^fps_limit=.*/fps_limit=" + value + "/' \"$f\"; else printf '%s\\n' 'fps_limit=" + value + "' >> \"$f\"; fi; pkill -SIGUSR2 mangohud 2>/dev/null || true"]
        setter.running = true
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 14
        spacing: 8
        RowLayout {
            Layout.fillWidth: true
            TextField {
                id: field
                Layout.fillWidth: true
                placeholderText: Translation.tr("Set FPS limit")
                inputMethodHints: Qt.ImhDigitsOnly
                onAccepted: root.applyLimit()
            }
            IrisButton {
                text: Translation.tr("Apply")
                emphasized: true
                onClicked: root.applyLimit()
            }
        }
        IrisText {
            Layout.fillWidth: true
            text: root.status.length > 0 ? root.status : Translation.tr("0 disables the MangoHud frame limit")
            color: root.status.length > 0 ? OverlayLook.colError : OverlayLook.colSubtext
            font.pixelSize: Appearance.font.pixelSize.small
        }
    }

    Process {
        id: setter
        onExited: (code, status) => root.status = code === 0 ? Translation.tr("Saved") : Translation.tr("Could not save the limit")
    }
}
