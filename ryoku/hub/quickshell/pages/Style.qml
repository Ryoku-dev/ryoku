import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/style"
import "../schema/style.js" as Schema

// how every Ryoku surface looks: the design, chosen from tiles over a live, usable preview,
// then the colours every design wears (Ryoku's or the desktop's, light or dark, contrast)
Item {
    id: root

    required property var hub

    readonly property var designRow: Schema.rows.find(r => r.id === "design")
    readonly property string chosen: {
        Changes.revision;
        var v = Changes.value(root.designRow.bind);
        return Schema.designs.some(d => d.value === v) ? v : "lithic";
    }
    // the design the preview shows: the last tile rested on, so the pointer can go down and
    // try it, else the one in use; choosing one shows it again
    property string previewing: ""
    onChosenChanged: root.previewing = ""
    readonly property string shown: root.previewing !== "" ? root.previewing : root.chosen

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    function choose(id) {
        var d = Schema.designs.find(x => x.value === id);
        Changes.set(root.designRow.bind, id, root.designRow.label, "style",
                    I18n.tr("Design set to %1").replace("%1", I18n.tr(d ? d.label : id)));
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "style"
        schema: Schema
        bodies: ({ design: gallery, scheme: palette })
        // only Lithic tones its surfaces from the wallpaper, so Tint is Lithic's alone
        adaptRows: rows => rows.filter(r => r.id !== "tint" || root.chosen === "lithic")
    }

    Component {
        id: gallery
        Column {
            spacing: 14

            Grid {
                id: grid
                width: parent.width
                readonly property int across: width >= 720 ? 4 : 2
                columns: across
                columnSpacing: 10
                rowSpacing: 10

                Repeater {
                    model: Schema.designs
                    delegate: StyleChoice {
                        required property var modelData
                        width: Math.floor((grid.width - (grid.across - 1) * grid.columnSpacing) / grid.across)
                        design: modelData.value
                        text: I18n.tr(modelData.label)
                        detail: I18n.tr(modelData.detail)
                        chosen: root.chosen === modelData.value
                        onClicked: root.choose(modelData.value)
                        onHoveredChanged: {
                            if (hovered)
                                root.previewing = modelData.value;
                        }
                    }
                }
            }

            // the preview is an example: say so before it, so its switches never read as settings
            Row {
                width: parent.width
                spacing: 10
                CutRect {
                    id: chip
                    anchors.verticalCenter: parent.verticalCenter
                    width: chipLabel.implicitWidth + 16
                    height: 22
                    cut: LithicTokens.cut.small
                    fill: "transparent"
                    strokeWidth: 1
                    strokeColor: Lithic.fg3
                    Label {
                        id: chipLabel
                        anchors.centerIn: parent
                        text: I18n.tr("Example")
                        typeRole: "group"
                        inkRole: "fg2"
                        font.capitalization: Font.AllUppercase
                        font.letterSpacing: 1.2
                    }
                }
                Label {
                    id: caption
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - chip.width - parent.spacing
                    text: (root.shown === root.chosen
                        ? I18n.tr("How Settings looks in %1, the design in use.")
                        : I18n.tr("How Settings would look in %1. Choose its tile to use it."))
                        .replace("%1", I18n.tr(Schema.designs.find(d => d.value === root.shown).label))
                        + " " + I18n.tr("Its controls only show the design; they change nothing.")
                    typeRole: "caption"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }

            StylePreview {
                id: preview
                width: parent.width
                height: implicitHeight
                design: root.shown
                opacity: 1
                onDesignChanged: swap.restart()
                SequentialAnimation {
                    id: swap
                    NumberAnimation { target: preview; property: "opacity"; from: 0.35; to: 1; duration: Tone.move; easing.type: Easing.OutCubic }
                }
            }
        }
    }

    Component {
        id: palette
        PaletteStrip {}
    }
}
