pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// In-app settings: where machines live, the engine status, the new-machine
// defaults, and a one-click install when quickemu is missing. Overlay spec:
// paperRaised fill, lineStrong border, radius 2, scrim black 55%.
Item {
    id: sp
    property bool open: false
    signal closed()

    visible: opacity > 0
    opacity: open ? 1 : 0
    Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }

    // scrim: only an outside click (or the ✕) dismisses the panel
    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.55)
        MouseArea { anchors.fill: parent; onClicked: sp.closed() }
    }

    Rectangle {
        anchors.centerIn: parent
        width: 480
        height: col.implicitHeight + 2 * Tokens.s5
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent }   // a click inside the panel never dismisses it

        Column {
            id: col
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s5
            spacing: Tokens.s5

            Item {
                width: parent.width
                height: 28
                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Settings")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: 18
                    font.weight: Font.DemiBold
                }
                IconBtn {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "\u2715"
                    onAct: sp.closed()
                }
            }

            // engine status.
            Item {
                width: parent.width
                height: 40
                Column {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2
                    Text { text: I18n.tr("Engine"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 13; font.weight: Font.Medium }
                    Text {
                        text: Vm.caps.quickemu ? ("quickemu " + (Vm.caps.version || "")) : I18n.tr("quickemu not installed")
                        color: Vm.caps.quickemu ? Tokens.inkDim : Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: 11
                    }
                }
                Btn {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: !Vm.caps.quickemu
                    primary: true
                    text: I18n.tr("INSTALL")
                    onAct: sp.installEngine()
                }
            }

            Rectangle { width: parent.width; height: 1; color: Tokens.line }

            Section {
                id: defs
                width: parent.width
                title: I18n.tr("New machine defaults")
                Cell {
                    width: defs.span(Spans.of("step"))
                    controlWidth: Spans.inlineWidth("step", 0, width)
                    label: I18n.tr("CPU cores")
                    value: String(Vm.settings.defaultCores)
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Vm.settings.defaultCores
                        from: 1; to: 32
                        onModified: (v) => { Vm.settings.defaultCores = v; Vm.saveSettings(); }
                    }
                }
                Cell {
                    width: defs.span(Spans.of("step"))
                    controlWidth: Spans.inlineWidth("step", 0, width)
                    label: I18n.tr("Memory")
                    unit: "GB"
                    value: String(Vm.settings.defaultRam)
                    Step {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        value: Vm.settings.defaultRam
                        from: 1; to: 128
                        onModified: (v) => { Vm.settings.defaultRam = v; Vm.saveSettings(); }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: Tokens.line }

            // machines path.
            Item {
                width: parent.width
                height: 38
                Column {
                    anchors.left: parent.left
                    anchors.right: openBtn.left
                    anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2
                    Text { text: I18n.tr("Machines"); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 13; font.weight: Font.Medium }
                    Text {
                        width: parent.width
                        elide: Text.ElideMiddle
                        text: Vm.paths.vms || "~/.local/share/ryoku/vms"
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 11
                    }
                }
                Btn {
                    id: openBtn
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("OPEN")
                    onAct: Vm.openFolder("")
                }
            }

            Rectangle { width: parent.width; height: 1; color: Tokens.line }

            // catalogue provenance.
            Column {
                width: parent.width
                spacing: Tokens.s2
                Text {
                    text: I18n.tr("Catalogue")
                    color: Tokens.ink
                    font.family: Tokens.ui; font.pixelSize: 11; font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark; font.capitalization: Font.AllUppercase
                }
                ProvRow { k: I18n.tr("Source"); v: (Vm.paths.provider || "quickemu + quickget") + " · " + (Vm.paths.source || "github.com/quickemu-project") }
                ProvRow { k: I18n.tr("Logos"); v: Vm.paths.icons_provider || "simple-icons + quickemu-icons" }
                ProvRow { k: I18n.tr("Cached"); v: Vm.paths.icons || "~/.cache/ryoku/ryovm-icons" }
            }
        }
    }

    component ProvRow: Item {
        property string k: ""
        property string v: ""
        width: parent ? parent.width : 0
        height: 18
        Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; width: 64; text: parent.k; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 12 }
        Text { anchors.left: parent.left; anchors.leftMargin: 70; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideMiddle; text: parent.v; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 11 }
    }

    function installEngine() {
        Quickshell.execDetached(["sh", "-c",
            "exec \"${TERMINAL:-kitty}\" --class=ryovm -e sh -c \"ryovm setup; echo; read -n1 -rsp 'Press any key to close…'; echo\""]);
    }
}
