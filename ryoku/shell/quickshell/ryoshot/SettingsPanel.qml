import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import "Singletons"
import Ryoku.Ui.Singletons

Item {
    id: panel

    property alias defaultChord: hotkeyRow.defaultChord
    property alias hotkey: hotkeyRow.hotkey

    signal closeRequested()
    signal rebound()

    readonly property int arrow: 7
    implicitWidth: card.implicitWidth
    implicitHeight: card.implicitHeight + arrow

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

    Rectangle {
        id: card
        width: parent.width
        height: parent.height - panel.arrow
        radius: Theme.radius
        color: Theme.panel
        border.color: Theme.hair
        border.width: 1
        implicitWidth: 300
        implicitHeight: content.implicitHeight + 28

        ColumnLayout {
            id: content
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: 14
            spacing: 12

            HotkeyRow {
                id: hotkeyRow
                Layout.fillWidth: true
                onRebound: panel.rebound()
                onCloseRequested: panel.closeRequested()
            }

            Rectangle { Layout.fillWidth: true; height: 1; color: Theme.hair }

            Slider {
                Layout.fillWidth: true
                label: I18n.tr("Blur radius")
                from: 16
                to: 128
                value: Config.blurRadius
                onMoved: (v) => Config.blurRadius = Math.round(v / 4) * 4
                onReleased: Config.save()
            }

            Slider {
                Layout.fillWidth: true
                label: I18n.tr("Mosaic block")
                from: 6
                to: 32
                value: Config.mosaicBlock
                onMoved: (v) => Config.mosaicBlock = Math.round(v)
                onReleased: Config.save()
            }

            Slider {
                Layout.fillWidth: true
                label: I18n.tr("Zoom factor")
                from: 1.5
                to: 4.0
                decimals: 1
                value: Config.zoomFactor
                onMoved: (v) => Config.zoomFactor = Math.round(v * 10) / 10
                onReleased: Config.save()
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 10

                Text {
                    Layout.fillWidth: true
                    text: I18n.tr("Copy on save")
                    color: Theme.inkDim
                    font.family: Theme.ui
                    font.pixelSize: 12
                    font.weight: Font.Medium
                }

                Rectangle {
                    id: pill
                    width: 44
                    height: 24
                    radius: 12
                    color: Config.copyOnSave ? Theme.accent : Theme.hover
                    border.color: Config.copyOnSave ? Theme.accent : Theme.hair
                    border.width: 1

                    Rectangle {
                        width: 18
                        height: 18
                        radius: 9
                        anchors.verticalCenter: parent.verticalCenter
                        x: Config.copyOnSave ? parent.width - width - 3 : 3
                        color: Config.copyOnSave ? Theme.accentInk : Theme.ink
                        Behavior on x { NumberAnimation { duration: Theme.move } }

                        Icon {
                            anchors.centerIn: parent
                            name: "check"
                            size: 12
                            tint: Theme.accent
                            visible: Config.copyOnSave
                        }
                    }

                    MouseArea {
                        anchors.fill: parent
                        onClicked: { Config.copyOnSave = !Config.copyOnSave; Config.save(); }
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 10

                Text {
                    text: I18n.tr("Save folder")
                    color: Theme.inkDim
                    font.family: Theme.ui
                    font.pixelSize: 12
                    font.weight: Font.Medium
                }

                Text {
                    Layout.fillWidth: true
                    text: Config.saveDir.length > 0 ? Config.saveDir : I18n.tr("Default")
                    color: Theme.ink
                    font.family: Theme.mono
                    font.pixelSize: 12
                    elide: Text.ElideMiddle
                    horizontalAlignment: Text.AlignRight
                }

                Rectangle {
                    id: chooseBtn
                    Layout.preferredHeight: 28
                    Layout.preferredWidth: chooseLabel.implicitWidth + 24
                    radius: 6
                    color: chooseHover.hovered ? Theme.press : Theme.hover
                    border.color: Theme.hair
                    border.width: 1

                    Text {
                        id: chooseLabel
                        anchors.centerIn: parent
                        text: I18n.tr("Choose")
                        color: Theme.inkDim
                        font.family: Theme.mono
                        font.pixelSize: 12
                    }

                    HoverHandler { id: chooseHover }
                    TapHandler { onTapped: folderDialog.open() }
                }
            }

            Rectangle { Layout.fillWidth: true; height: 1; color: Theme.hair }

            RowLayout {
                Layout.fillWidth: true
                spacing: 10

                Text {
                    Layout.fillWidth: true
                    text: I18n.tr("Recording settings")
                    color: Theme.inkDim
                    font.family: Theme.ui
                    font.pixelSize: 12
                    font.weight: Font.Medium
                }

                Rectangle {
                    id: recordingBtn
                    Layout.preferredHeight: 28
                    Layout.preferredWidth: recordingLabel.implicitWidth + 24
                    radius: 6
                    color: recordingHover.hovered ? Theme.press : Theme.hover
                    border.color: Theme.hair
                    border.width: 1

                    Text {
                        id: recordingLabel
                        anchors.centerIn: parent
                        text: I18n.tr("Open")
                        color: Theme.inkDim
                        font.family: Theme.mono
                        font.pixelSize: 12
                    }

                    HoverHandler { id: recordingHover }
                    TapHandler {
                        onTapped: {
                            Bus.run("hub open recording");
                            panel.closeRequested();
                        }
                    }
                }
            }
        }
    }

    Canvas {
        width: panel.arrow * 2
        height: panel.arrow
        anchors.top: card.bottom
        anchors.horizontalCenter: card.horizontalCenter
        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            ctx.beginPath();
            ctx.moveTo(0, 0);
            ctx.lineTo(width, 0);
            ctx.lineTo(width / 2, height);
            ctx.closePath();
            ctx.fillStyle = Theme.panel;
            ctx.fill();
        }
    }
}
