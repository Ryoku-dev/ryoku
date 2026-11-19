pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "Singletons"
import "look"

Flyout {
    id: panel

    property alias defaultChord: hotkeyRow.defaultChord
    property alias hotkey: hotkeyRow.hotkey

    signal rebound()

    materialRadius: 28

    Process {
        id: folderDialog
        stdout: StdioCollector { id: folderOut }
        function open() {
            command = ["sh", "-c",
                "zenity --file-selection --directory 2>/dev/null || kdialog --getexistingdirectory ~ 2>/dev/null"];
            running = true;
        }
        onExited: (code) => {
            var chosen = folderOut.text.trim();
            if (code === 0 && chosen.length > 0) {
                Config.saveDir = chosen;
                Config.save();
            }
        }
    }

    component RowLabel: Text {
        color: Look.fg
        font.family: Look.sans
        font.pixelSize: Math.round(13 * panel.u)
        renderType: Text.CurveRendering
    }

    ColumnLayout {
        width: Math.round(300 * panel.u)
        spacing: Math.round(12 * panel.u)

        Text {
            text: Look.lithic ? I18n.tr("Settings").toUpperCase() : I18n.tr("Settings")
            color: Look.lithic ? Look.fg3 : Look.fg
            font.family: Look.lithic ? Look.mono : Look.sans
            font.pixelSize: Math.round((Look.lithic ? 9 : Look.material ? 16 : 14) * panel.u)
            font.weight: Look.lithic ? Font.Bold : Font.DemiBold
            font.letterSpacing: Look.lithic ? Look.tracking * panel.u : 0
            renderType: Text.CurveRendering
        }

        HotkeyRow {
            id: hotkeyRow
            Layout.fillWidth: true
            u: panel.u
            onRebound: panel.rebound()
            onCloseRequested: panel.closeRequested()
        }

        Divider { Layout.fillWidth: true; u: panel.u; vertical: false }

        Slider {
            Layout.fillWidth: true
            u: panel.u
            label: I18n.tr("Blur radius")
            from: 16
            to: 128
            value: Config.blurRadius
            onMoved: (v) => Config.blurRadius = Math.round(v / 4) * 4
            onReleased: Config.save()
        }

        Slider {
            Layout.fillWidth: true
            u: panel.u
            label: I18n.tr("Mosaic block")
            from: 6
            to: 32
            value: Config.mosaicBlock
            onMoved: (v) => Config.mosaicBlock = Math.round(v)
            onReleased: Config.save()
        }

        Slider {
            Layout.fillWidth: true
            u: panel.u
            label: I18n.tr("Zoom factor")
            from: 1.5
            to: 4.0
            decimals: 1
            value: Config.zoomFactor
            onMoved: (v) => Config.zoomFactor = Math.round(v * 10) / 10
            onReleased: Config.save()
        }

        Divider { Layout.fillWidth: true; u: panel.u; vertical: false }

        RowLayout {
            Layout.fillWidth: true
            spacing: Math.round(10 * panel.u)

            RowLabel {
                Layout.fillWidth: true
                text: I18n.tr("Copy on save")
            }
            Switch {
                u: panel.u
                checked: Config.copyOnSave
                onToggled: (v) => { Config.copyOnSave = v; Config.save(); }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: Math.round(10 * panel.u)

            ColumnLayout {
                Layout.fillWidth: true
                spacing: Math.round(2 * panel.u)
                RowLabel {
                    text: I18n.tr("Save folder")
                }
                Text {
                    Layout.fillWidth: true
                    text: Config.saveDir.length > 0 ? Config.saveDir : I18n.tr("Default")
                    color: Look.fg2
                    font.family: Look.mono
                    font.pixelSize: Math.round(10.5 * panel.u)
                    elide: Text.ElideMiddle
                    renderType: Text.CurveRendering
                }
            }
            TextButton {
                u: panel.u
                cellHeight: 28
                text: I18n.tr("Choose")
                onClicked: folderDialog.open()
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: Math.round(10 * panel.u)

            RowLabel {
                Layout.fillWidth: true
                text: I18n.tr("Recording settings")
            }
            TextButton {
                u: panel.u
                cellHeight: 28
                text: I18n.tr("Open")
                onClicked: {
                    Bus.run("hub open screen-capture");
                    panel.closeRequested();
                }
            }
        }
    }
}
