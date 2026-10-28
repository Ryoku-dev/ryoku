import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import "Singletons"
import Ryoku.Ui.Singletons

Item {
    id: tb
    implicitWidth: glass.implicitWidth
    implicitHeight: glass.implicitHeight

    property var tools: []
    property string activeTool: "rect"
    property color activeColor: Theme.accent
    property int activeWidth: 4
    property bool activeFill: false
    property bool activeRough: false
    property bool hasFill: false
    property string openPopover: ""
    property bool canUndo: false
    property bool canRedo: false
    property bool settingsOpen: false
    property bool phoneAvailable: false
    property var phoneDevices: []
    property bool phoneReady: false
    property bool phoneBusy: false

    readonly property real colorCenterX: colorBtn.x + row.x + colorBtn.width / 2
    readonly property real widthCenterX: widthBtn.x + row.x + widthBtn.width / 2
    readonly property real phoneCenterX: phoneBtn.x + row.x + phoneBtn.width / 2
    readonly property real gearCenterX: gear.x + row.x + gear.width / 2

    signal toolPicked(string tool)
    signal colorButtonClicked()
    signal widthButtonClicked()
    signal fillToggled()
    signal roughToggled()
    signal undoRequested()
    signal redoRequested()
    signal copyRequested()
    signal saveRequested()
    signal uploadRequested()
    signal phoneRequested()
    signal pinRequested()
    signal helpRequested()
    signal settingsRequested()
    signal beautifyRequested()
    signal dragged(real dx, real dy)

    // the desktop brand seal, user-overridable via ~/.config/ryoku/brand.json
    // (Ryoku Settings -> Shell -> Global). the beautify button follows markText;
    // empty/absent falls back to the 力 default, so this never seeds.
    readonly property string mark: UiPreferences.brandMark(brandAdapter.markText)
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: brandAdapter; property string markText: "力" }
    }

    function keyFor(id) {
        for (var i = 0; i < tb.tools.length; i++)
            if (tb.tools[i].id === id) return tb.tools[i].key;
        return "";
    }

    Rectangle {
        id: glass
        anchors.fill: parent
        radius: Theme.radius
        color: Theme.panel
        border.color: Theme.hair
        border.width: 1
        implicitWidth: row.implicitWidth + 12
        implicitHeight: row.implicitHeight + 12

        RowLayout {
            id: row
            anchors.centerIn: parent
            spacing: 2

            // Deltas are reported in window space: the bar moves under the
            // pointer as it is dragged, so a delta measured against the bar's
            // own coordinates would feed back on itself.
            Item {
                id: grip
                Layout.preferredWidth: 14
                Layout.preferredHeight: 32

                Column {
                    anchors.centerIn: parent
                    spacing: 3
                    Repeater {
                        model: 3
                        Rectangle {
                            width: 3
                            height: 3
                            radius: 1.5
                            color: gripMa.containsMouse || gripMa.pressed ? Theme.ink : Theme.inkFaint
                        }
                    }
                }

                MouseArea {
                    id: gripMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                    property real lastX: 0
                    property real lastY: 0
                    onPressed: (m) => {
                        var p = mapToItem(null, m.x, m.y);
                        lastX = p.x;
                        lastY = p.y;
                    }
                    onPositionChanged: (m) => {
                        if (!pressed) return;
                        var p = mapToItem(null, m.x, m.y);
                        tb.dragged(p.x - lastX, p.y - lastY);
                        lastX = p.x;
                        lastY = p.y;
                    }
                }
            }

            Repeater {
                model: tb.tools
                IconButton {
                    required property var modelData
                    icon: modelData.icon
                    active: tb.activeTool === modelData.id
                    tooltip: modelData.label + "  (" + modelData.key + ")"
                    onClicked: tb.toolPicked(modelData.id)
                }
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 20; color: Theme.hair; Layout.leftMargin: 3; Layout.rightMargin: 3 }

            Rectangle {
                id: colorBtn
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
                radius: 7
                color: tb.openPopover === "color" ? Theme.press : (colorMa.containsMouse ? Theme.hover : "transparent")
                Rectangle {
                    anchors.centerIn: parent
                    width: 18
                    height: 18
                    radius: 9
                    color: tb.activeColor
                    border.color: Theme.hair
                    border.width: 1
                }
                MouseArea {
                    id: colorMa
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: tb.colorButtonClicked()
                }
            }

            Rectangle {
                id: widthBtn
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
                radius: 7
                color: tb.openPopover === "width" ? Theme.press : (widthMa.containsMouse ? Theme.hover : "transparent")
                Rectangle {
                    anchors.centerIn: parent
                    width: Math.max(4, Math.min(18, tb.activeWidth * 1.6 + 3))
                    height: width
                    radius: width / 2
                    color: Theme.inkDim
                }
                MouseArea {
                    id: widthMa
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: tb.widthButtonClicked()
                }
            }

            IconButton {
                icon: "fill"
                visible: tb.hasFill
                active: tb.activeFill
                tooltip: I18n.tr("Fill") + "  (f)"
                onClicked: tb.fillToggled()
            }

            IconButton {
                icon: "sketch"
                active: tb.activeRough
                tooltip: I18n.tr("Sketch") + "  (k)"
                onClicked: tb.roughToggled()
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 20; color: Theme.hair; Layout.leftMargin: 3; Layout.rightMargin: 3 }

            IconButton { icon: "undo"; dim: !tb.canUndo; tooltip: I18n.tr("Undo") + "  (ctrl+z)"; onClicked: { if (tb.canUndo) tb.undoRequested(); } }
            IconButton { icon: "redo"; dim: !tb.canRedo; tooltip: I18n.tr("Redo") + "  (ctrl+shift+z)"; onClicked: { if (tb.canRedo) tb.redoRequested(); } }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 20; color: Theme.hair; Layout.leftMargin: 3; Layout.rightMargin: 3 }

            IconButton { icon: "copy"; tooltip: I18n.tr("Copy") + "  (ctrl+c)"; onClicked: tb.copyRequested() }
            IconButton { icon: "save"; tooltip: I18n.tr("Save") + "  (ctrl+s)"; onClicked: tb.saveRequested() }
            IconButton { icon: "pin"; tooltip: I18n.tr("Pin to desktop") + "  (ctrl+p)"; onClicked: tb.pinRequested() }
            IconButton { icon: "upload"; tooltip: I18n.tr("Upload") + "  (ctrl+u)"; onClicked: tb.uploadRequested() }
            IconButton {
                id: phoneBtn
                visible: tb.phoneAvailable
                icon: "phone"
                dim: tb.phoneBusy || !tb.phoneReady || tb.phoneDevices.length === 0
                tooltip: tb.phoneBusy ? I18n.tr("Sending screenshot")
                    : !tb.phoneReady ? I18n.tr("Phone transfer is unavailable")
                    : tb.phoneDevices.length === 0 ? I18n.tr("No connected phone")
                    : tb.phoneDevices.length === 1
                        ? I18n.tr("Send to %1").arg(tb.phoneDevices[0].name) + "  (ctrl+k)"
                        : I18n.tr("Choose a phone") + "  (ctrl+k)"
                onClicked: tb.phoneRequested()
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 20; color: Theme.hair; Layout.leftMargin: 3; Layout.rightMargin: 3 }

            Rectangle {
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
                radius: 7
                color: beautMa.containsMouse ? Theme.hover : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: tb.mark
                    color: Theme.accent
                    font.family: Theme.jp
                    font.pixelSize: 17
                    font.weight: Font.DemiBold
                }
                MouseArea { id: beautMa; anchors.fill: parent; hoverEnabled: true; onClicked: tb.beautifyRequested() }
            }

            Rectangle { Layout.preferredWidth: 1; Layout.preferredHeight: 20; color: Theme.hair; Layout.leftMargin: 3; Layout.rightMargin: 3 }

            Rectangle {
                id: helpBtn
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
                radius: 7
                color: helpMa.containsMouse ? Theme.hover : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: "?"
                    color: Theme.inkDim
                    font.family: Theme.ui
                    font.pixelSize: 16
                    font.weight: Font.DemiBold
                }
                MouseArea { id: helpMa; anchors.fill: parent; hoverEnabled: true; onClicked: tb.helpRequested() }
            }

            IconButton {
                id: gear
                icon: "gear"
                active: tb.settingsOpen
                tooltip: I18n.tr("Settings")
                onClicked: tb.settingsRequested()
            }
        }
    }
}
