import QtQuick
import "../../modules"
import Ryoku.Ui.Singletons

// The workspaces widget's live marker preview, lifted out of the old Spaces route
// so the Widgets route can show it under the workspaces settings without
// re-rolling it. A bar-like pill wrapping the marker row, drawn in the bar's own
// colours and sizes (i.e. as data): a preview that drifts from the bar is worse
// than none. `root` is the qsbar Theme; `tk` its geometry.
Item {
    id: prev
    property var root: null
    property var tk: null

    implicitWidth: prev.width
    implicitHeight: 44 + (prev.tk ? prev.tk.gap * 2 : 24)

    // Sample ids: "active" shows a representative trio; the fixed modes show the
    // full 1..N range. Focused = 1, occupied = 2/3, the rest empty.
    readonly property var previewList: {
        if (!prev.root) return []
        var m = prev.root.workspaceMode
        if (m === "active") return [1, 2, 3]
        var n = m === "5" ? 5 : 10
        var list = []
        for (var j = 1; j <= n; j++) list.push(j)
        return list
    }

    Rectangle {
        anchors.left: parent.left
        anchors.leftMargin: prev.tk ? prev.tk.gap : 12
        anchors.verticalCenter: parent.verticalCenter
        height: 44
        width: Math.round(previewRow.implicitWidth) + (prev.tk ? prev.tk.pad : 24)
        radius: prev.root ? prev.root.pillRadius : 12
        color: prev.root ? prev.root.pill : "transparent"
        border.width: 1
        border.color: prev.root ? prev.root.sep : "transparent"

        Row {
            id: previewRow
            anchors.centerIn: parent
            spacing: 5

            Repeater {
                model: prev.previewList

                delegate: Item {
                    id: cell
                    required property int modelData
                    required property int index
                    readonly property string style: prev.root ? prev.root.workspaceStyle : "default"
                    readonly property color seal: prev.root ? prev.root.seal : "#888888"
                    readonly property bool isFocused: modelData === 1
                    readonly property bool isOccupied: modelData === 2 || modelData === 3
                    readonly property bool isEmpty: !isFocused && !isOccupied

                    implicitWidth: style === "numbers" ? 22
                                 : style === "icons"   ? 22
                                 : style === "magic"   ? (isFocused ? 20 : 18)
                                 : style === "kanji"   ? 22
                                 : style === "rings"   ? 20
                                 : style === "aurora"  ? (isFocused ? 34 : 12)
                                 : style === "pacman"  ? 22
                                 : (isFocused ? 32 : 16)
                    implicitHeight: 28

                    Behavior on implicitWidth { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }

                    // DEFAULT - glow + dot
                    Rectangle {
                        visible: cell.style === "default"
                        anchors.centerIn: parent
                        width: cell.isFocused ? 34 : 16
                        height: 16
                        radius: 8
                        color: cell.isFocused  ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.20)
                             : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.18)
                                               : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.06)
                        Behavior on width { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
                        Behavior on color { ColorAnimation { duration: 200 } }
                    }
                    Rectangle {
                        visible: cell.style === "default"
                        anchors.centerIn: parent
                        width: cell.isFocused ? 26 : 8
                        height: 8
                        radius: 4
                        color: cell.isEmpty ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.25) : cell.seal
                        Behavior on width { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
                        Behavior on color { ColorAnimation { duration: 200 } }
                    }

                    // NUMBERS - digit on a rounded badge; ICONS puts the symbol there
                    Rectangle {
                        id: numberBadge
                        readonly property string icon: cell.style === "icons" && prev.root
                            ? prev.root.workspaceIcon(cell.modelData) : ""
                        visible: cell.style === "numbers" || cell.style === "icons"
                        anchors.centerIn: parent
                        width: 20
                        height: 20
                        radius: height / 2
                        clip: true
                        color: cell.isFocused  ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.30)
                             : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.12)
                                               : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.04)
                        Behavior on color { ColorAnimation { duration: 200 } }
                        IconText {
                            visible: numberBadge.icon !== ""
                            anchors.centerIn: parent
                            text: numberBadge.icon
                            fill: cell.isFocused ? 1 : 0
                            color: cell.isFocused  ? cell.seal
                                 : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.5)
                                                   : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.28)
                            font.pixelSize: cell.isFocused ? 16 : 15
                            font.weight: cell.isFocused ? Font.Bold : Font.Normal
                        }
                        UiText {
                            visible: numberBadge.icon === ""
                            anchors.centerIn: parent
                            text: cell.modelData
                            color: cell.isFocused  ? Qt.lighter(cell.seal, 1.3)
                                 : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.5)
                                                   : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.28)
                            font.family: prev.root ? prev.root.mono : "monospace"
                            font.pixelSize: cell.isFocused ? 13 : 12
                            font.weight: cell.isFocused ? Font.Bold : Font.Normal
                        }
                    }

                    // MAGIC - sparkle glyphs (filled / hollow / dot)
                    Text {
                        visible: cell.style === "magic"
                        anchors.centerIn: parent
                        anchors.verticalCenterOffset: cell.isFocused ? 0 : 1
                        text: cell.isFocused  ? String.fromCodePoint(0x2726)
                            : cell.isOccupied ? String.fromCodePoint(0x2727)
                                              : String.fromCodePoint(0x00B7)
                        color: cell.isFocused  ? cell.seal
                             : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.7)
                                               : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.3)
                        font.family: "Adwaita Mono"
                        font.pixelSize: cell.isFocused ? 22 : 18
                        renderType: Text.NativeRendering
                        Behavior on color { ColorAnimation { duration: 200 } }
                    }

                    // KANJI - numerals
                    Text {
                        visible: cell.style === "kanji"
                        anchors.centerIn: parent
                        text: ["\u4e00", "\u4e8c", "\u4e09", "\u56db", "\u4e94"][cell.index] || ""
                        color: cell.isFocused  ? cell.seal
                             : cell.isOccupied ? Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.7)
                                               : Qt.rgba(cell.seal.r, cell.seal.g, cell.seal.b, 0.3)
                        font.family: "Noto Sans CJK JP"
                        font.pixelSize: cell.isFocused ? 15 : 13
                        renderType: Text.NativeRendering
                    }

                    // RINGS - numerals with an outline behind the focused cell
                    Rectangle {
                        visible: cell.style === "rings"
                        anchors.centerIn: parent
                        width: 20
                        height: 20
                        radius: prev.root ? prev.root.tileRadius : 6
                        color: "transparent"
                        border.width: cell.isFocused ? 1 : 0
                        border.color: cell.seal
                    }
                    Text {
                        visible: cell.style === "rings"
                        anchors.centerIn: parent
                        text: String(cell.index + 1)
                        color: cell.seal
                        opacity: cell.isFocused ? 1.0 : cell.isOccupied ? 0.64 : 0.24
                        font.family: prev.root ? prev.root.mono : "monospace"
                        font.pixelSize: 12
                        renderType: Text.QtRendering
                    }

                    // AURORA - one flat light streak
                    Rectangle {
                        visible: cell.style === "aurora"
                        anchors.centerIn: parent
                        width: cell.isFocused ? 28 : (cell.isOccupied ? 6 : 4)
                        height: cell.isFocused ? 3 : (cell.isOccupied ? 6 : 4)
                        radius: height / 2
                        color: cell.seal
                        opacity: cell.isFocused ? 0.92 : cell.isOccupied ? 0.62 : 0.18
                        antialiasing: true
                    }

                    // PACMAN - resting pacman on focused, pellets elsewhere
                    PacmanMarker {
                        visible: cell.style === "pacman"
                        anchors.centerIn: parent
                        focused: cell.isFocused
                        occupied: cell.isOccupied
                        activeColor: cell.seal
                        occupiedColor: cell.seal
                        emptyColor: cell.seal
                        hoverColor: cell.seal
                    }
                }
            }
        }
    }
}
